package prompts

import "encoding/json"

var EventAnalysisSchema = json.RawMessage(`{
  "type":"object",
  "additionalProperties":false,
  "required":["summary","activity","persons","vehicle_present","package_present","risk","confidence"],
  "properties":{
    "summary":{"type":"string","maxLength":1000},
    "activity":{"type":"string","maxLength":128},
    "persons":{"type":"integer","minimum":0,"maximum":100},
    "vehicle_present":{"type":"boolean"},
    "package_present":{"type":"boolean"},
    "risk":{"type":"string","enum":["unknown","low","medium","high"]},
    "confidence":{"type":"number","minimum":0,"maximum":1}
  }
}`)

const EventAnalysisPrompt = `Analyze only what is visibly supported by the supplied security-camera frames. Return the requested structured JSON. Do not identify a person by name. Do not infer protected/sensitive traits. Use risk=unknown when evidence is insufficient.`
