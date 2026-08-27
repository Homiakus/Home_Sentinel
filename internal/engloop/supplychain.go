package engloop

import (
	"bufio"
	"fmt"
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
		findings, err := verifyWorkflowActions(filepath.Join(workflowRoot, entry.Name()), rel)
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

func verifyWorkflowActions(path, rel string) ([]Finding, error) {
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
		if strings.HasPrefix(value, "./") || strings.HasPrefix(value, "docker://") {
			continue
		}
		at := strings.LastIndexByte(value, '@')
		if at <= 0 || at == len(value)-1 {
			findings = append(findings, Finding{ID: "action-ref-missing", Severity: SeverityBlocker, Path: rel, Message: fmt.Sprintf("line %d external action has no immutable ref: %s", lineNo, value)})
			continue
		}
		ref := value[at+1:]
		if !fullCommitSHA.MatchString(ref) {
			findings = append(findings, Finding{ID: "action-ref-mutable", Severity: SeverityBlocker, Path: rel, Message: fmt.Sprintf("line %d external action must use a 40-character commit SHA, got %q", lineNo, ref)})
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
