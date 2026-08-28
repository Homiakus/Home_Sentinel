package engloop

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	PinnedGremlinsVersion    = "v0.6.0"
	PinnedGovulncheckVersion = "v1.7.0"
	PinnedCycloneDXVersion   = "v1.10.0"
	PinnedTrivyVersion       = "v0.74.0"
)

var fullCommitSHA = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)

type ActionPin struct {
	Version string `json:"version"`
	SHA     string `json:"sha"`
}

type ActionPinManifest struct {
	SchemaVersion int                  `json:"schema_version"`
	Actions       map[string]ActionPin `json:"actions"`
}

type SupplyChainReport struct {
	Root     string    `json:"root"`
	Findings []Finding `json:"findings"`
}

func (r SupplyChainReport) HasBlockers() bool {
	for _, finding := range r.Findings {
		if finding.Severity == SeverityBlocker {
			return true
		}
	}
	return false
}

// VerifySupplyChain validates repository-local supply-chain invariants that
// must remain true before CI is allowed to trust its own verification result.
func VerifySupplyChain(root string) (SupplyChainReport, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return SupplyChainReport{}, fmt.Errorf("resolve root: %w", err)
	}
	report := SupplyChainReport{Root: abs}

	pinsPath := filepath.Join(abs, "docs", "security", "CI_ACTION_PINS.json")
	pins, pinErr := loadActionPinManifest(pinsPath)
	if pinErr != nil {
		report.Findings = append(report.Findings, Finding{
			ID:       "action-pin-manifest-invalid",
			Severity: SeverityBlocker,
			Path:     "docs/security/CI_ACTION_PINS.json",
			Message:  pinErr.Error(),
		})
	}

	workflowRoot := filepath.Join(abs, ".github", "workflows")
	entries, err := os.ReadDir(workflowRoot)
	if err != nil {
		if os.IsNotExist(err) {
			report.Findings = append(report.Findings, Finding{ID: "workflow-root-missing", Severity: SeverityBlocker, Path: ".github/workflows", Message: "workflow directory is missing"})
			return report, nil
		}
		return SupplyChainReport{}, fmt.Errorf("read workflows: %w", err)
	}

	workflowCount := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".yml" && ext != ".yaml" {
			continue
		}
		workflowCount++
		rel := filepath.ToSlash(filepath.Join(".github", "workflows", entry.Name()))
		findings, err := verifyWorkflowActions(filepath.Join(workflowRoot, entry.Name()), rel, pins.Actions)
		if err != nil {
			return SupplyChainReport{}, err
		}
		report.Findings = append(report.Findings, findings...)
	}
	if workflowCount == 0 {
		report.Findings = append(report.Findings, Finding{ID: "workflow-set-empty", Severity: SeverityBlocker, Path: ".github/workflows", Message: "no workflow YAML files found"})
	}

	securityWorkflowPath := filepath.Join(workflowRoot, "security.yml")
	securityWorkflow, err := os.ReadFile(securityWorkflowPath)
	if err != nil {
		if os.IsNotExist(err) {
			report.Findings = append(report.Findings, Finding{ID: "security-workflow-missing", Severity: SeverityBlocker, Path: ".github/workflows/security.yml", Message: "SBOM and repository security qualification workflow is missing"})
		} else {
			return SupplyChainReport{}, fmt.Errorf("read security workflow: %w", err)
		}
	} else {
		requireExactPin(&report, "trivy-workflow-pin-drift", ".github/workflows/security.yml", string(securityWorkflow), "version: "+PinnedTrivyVersion)
	}

	dependabotPath := filepath.Join(abs, ".github", "dependabot.yml")
	if _, err := os.Stat(dependabotPath); err != nil {
		if os.IsNotExist(err) {
			report.Findings = append(report.Findings, Finding{ID: "dependabot-missing", Severity: SeverityBlocker, Path: ".github/dependabot.yml", Message: "dependency update automation is missing"})
		} else {
			return SupplyChainReport{}, fmt.Errorf("stat dependabot config: %w", err)
		}
	}

	makefile, err := os.ReadFile(filepath.Join(abs, "Makefile"))
	if err != nil {
		if os.IsNotExist(err) {
			report.Findings = append(report.Findings, Finding{ID: "makefile-missing", Severity: SeverityBlocker, Path: "Makefile", Message: "pinned engineering tool declarations are missing"})
		} else {
			return SupplyChainReport{}, fmt.Errorf("read Makefile: %w", err)
		}
	} else {
		text := string(makefile)
		requireExactPin(&report, "gremlins-pin-drift", "Makefile", text, "GREMLINS_VERSION := "+PinnedGremlinsVersion)
		requireExactPin(&report, "govulncheck-pin-drift", "Makefile", text, "GOVULNCHECK_VERSION := "+PinnedGovulncheckVersion)
		requireExactPin(&report, "cyclonedx-pin-drift", "Makefile", text, "CYCLONEDX_GOMOD_VERSION := "+PinnedCycloneDXVersion)
		if strings.Contains(text, "@latest") {
			report.Findings = append(report.Findings, Finding{ID: "floating-go-tool", Severity: SeverityBlocker, Path: "Makefile", Message: "go tool installation contains forbidden @latest reference"})
		}
	}

	policyPath := filepath.Join(abs, "docs", "security", "SUPPLY_CHAIN.md")
	policy, err := os.ReadFile(policyPath)
	if err != nil {
		if os.IsNotExist(err) {
			report.Findings = append(report.Findings, Finding{ID: "supply-chain-policy-missing", Severity: SeverityBlocker, Path: "docs/security/SUPPLY_CHAIN.md", Message: "supply-chain policy is missing"})
		} else {
			return SupplyChainReport{}, fmt.Errorf("read supply-chain policy: %w", err)
		}
	} else {
		text := string(policy)
		requireExactPin(&report, "govulncheck-policy-drift", "docs/security/SUPPLY_CHAIN.md", text, "golang.org/x/vuln/cmd/govulncheck@"+PinnedGovulncheckVersion)
		requireExactPin(&report, "cyclonedx-policy-drift", "docs/security/SUPPLY_CHAIN.md", text, "github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@"+PinnedCycloneDXVersion)
		requireExactPin(&report, "trivy-policy-drift", "docs/security/SUPPLY_CHAIN.md", text, "Trivy `"+PinnedTrivyVersion+"`")
		requireExactPin(&report, "action-pin-policy-unbound", "docs/security/SUPPLY_CHAIN.md", text, "CI_ACTION_PINS.json")
	}

	floating, err := scanExecutableLatestRefs(abs)
	if err != nil {
		return SupplyChainReport{}, err
	}
	report.Findings = append(report.Findings, floating...)

	if len(report.Findings) == 0 {
		report.Findings = append(report.Findings, Finding{ID: "supply-chain-clean", Severity: SeverityInfo, Message: "workflow actions, scanners, SBOM tooling and dependency automation match reviewed immutable policy"})
	}
	sort.SliceStable(report.Findings, func(i, j int) bool {
		if report.Findings[i].Severity != report.Findings[j].Severity {
			return report.Findings[i].Severity > report.Findings[j].Severity
		}
		if report.Findings[i].Path != report.Findings[j].Path {
			return report.Findings[i].Path < report.Findings[j].Path
		}
		return report.Findings[i].ID < report.Findings[j].ID
	})
	return report, nil
}

func loadActionPinManifest(path string) (ActionPinManifest, error) {
	file, err := os.Open(path)
	if err != nil {
		return ActionPinManifest{}, fmt.Errorf("open action pin manifest: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(io.LimitReader(file, 1<<20))
	decoder.DisallowUnknownFields()
	var manifest ActionPinManifest
	if err := decoder.Decode(&manifest); err != nil {
		return ActionPinManifest{}, fmt.Errorf("decode action pin manifest: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return ActionPinManifest{}, fmt.Errorf("decode action pin manifest: multiple JSON values")
		}
		return ActionPinManifest{}, fmt.Errorf("decode action pin manifest trailing data: %w", err)
	}
	if manifest.SchemaVersion != 1 {
		return ActionPinManifest{}, fmt.Errorf("unsupported schema_version %d", manifest.SchemaVersion)
	}
	if len(manifest.Actions) == 0 {
		return ActionPinManifest{}, fmt.Errorf("action allowlist is empty")
	}
	for name, pin := range manifest.Actions {
		if strings.TrimSpace(name) == "" || strings.TrimSpace(pin.Version) == "" {
			return ActionPinManifest{}, fmt.Errorf("action %q has an empty name or version", name)
		}
		if !fullCommitSHA.MatchString(pin.SHA) {
			return ActionPinManifest{}, fmt.Errorf("action %q has invalid SHA %q", name, pin.SHA)
		}
	}
	return manifest, nil
}

func verifyWorkflowActions(path, rel string, pins map[string]ActionPin) ([]Finding, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open workflow %s: %w", rel, err)
	}
	defer file.Close()

	var findings []Finding
	scanner := bufio.NewScanner(file)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		trimmed := strings.TrimSpace(scanner.Text())
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
		if !strings.HasPrefix(trimmed, "uses:") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(trimmed, "uses:"))
		if idx := strings.IndexByte(value, '#'); idx >= 0 {
			value = strings.TrimSpace(value[:idx])
		}
		value = strings.Trim(value, `"'`)
		if strings.HasPrefix(value, "./") {
			continue
		}
		if strings.HasPrefix(value, "docker://") {
			if !strings.Contains(value, "@sha256:") {
				findings = append(findings, Finding{ID: "docker-action-ref-mutable", Severity: SeverityBlocker, Path: rel, Message: fmt.Sprintf("line %d Docker action must use an immutable sha256 digest: %s", lineNo, value)})
			}
			continue
		}
		at := strings.LastIndexByte(value, '@')
		if at <= 0 || at == len(value)-1 {
			findings = append(findings, Finding{ID: "action-ref-missing", Severity: SeverityBlocker, Path: rel, Message: fmt.Sprintf("line %d external action has no immutable ref: %s", lineNo, value)})
			continue
		}
		actionName, ref := value[:at], value[at+1:]
		if !fullCommitSHA.MatchString(ref) {
			findings = append(findings, Finding{ID: "action-ref-mutable", Severity: SeverityBlocker, Path: rel, Message: fmt.Sprintf("line %d external action must use a 40-character commit SHA, got %q", lineNo, ref)})
			continue
		}
		pin, ok := pins[actionName]
		if !ok {
			findings = append(findings, Finding{ID: "action-not-reviewed", Severity: SeverityBlocker, Path: rel, Message: fmt.Sprintf("line %d action %s is absent from docs/security/CI_ACTION_PINS.json", lineNo, actionName)})
			continue
		}
		if !strings.EqualFold(pin.SHA, ref) {
			findings = append(findings, Finding{ID: "action-pin-drift", Severity: SeverityBlocker, Path: rel, Message: fmt.Sprintf("line %d action %s uses %s; reviewed %s pin is %s", lineNo, actionName, ref, pin.Version, pin.SHA)})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan workflow %s: %w", rel, err)
	}
	return findings, nil
}

func requireExactPin(report *SupplyChainReport, id, path, text, declaration string) {
	if !strings.Contains(text, declaration) {
		report.Findings = append(report.Findings, Finding{ID: id, Severity: SeverityBlocker, Path: path, Message: "required reviewed pin is absent: " + declaration})
	}
}

func scanExecutableLatestRefs(root string) ([]Finding, error) {
	var findings []Finding
	roots := []string{filepath.Join(root, ".github", "workflows"), filepath.Join(root, "scripts")}
	for _, scanRoot := range roots {
		if _, err := os.Stat(scanRoot); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		err := filepath.WalkDir(scanRoot, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if ext != ".yml" && ext != ".yaml" && ext != ".sh" && ext != ".bash" && ext != ".ps1" {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if strings.Contains(string(data), "go install") && strings.Contains(string(data), "@latest") {
				rel, _ := filepath.Rel(root, path)
				findings = append(findings, Finding{ID: "floating-go-tool", Severity: SeverityBlocker, Path: filepath.ToSlash(rel), Message: "executable automation contains forbidden go install @latest"})
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("scan executable automation: %w", err)
		}
	}
	return findings, nil
}
