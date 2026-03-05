package ai

import (
	"fmt"
	pb "github.com/MateuszBrankiewicz/cloudguardian/server/proto"
)

type Advisor struct {
	Ollama *OllamaClient
	Model  string
}

func NewAdvisor(ollama *OllamaClient, model string) *Advisor {
	if model == "" {
		model = "llama3"
	}
	return &Advisor{
		Ollama: ollama,
		Model:  model,
	}
}

func (a *Advisor) GenerateRiskReport(res *pb.InfrastructureResource, findings []PIIFinding) (string, error) {
	systemPrompt := `Jesteś ekspertem od Cyberbezpieczeństwa i FinOps. 
Twoim zadaniem jest analiza ryzyka i kosztów zasobów chmurowych.
Na podstawie dostarczonych danych technicznych i wykrytych danych wrażliwych (PII), 
oceń ryzyko w skali 1-10 i zaproponuj oszczędności oraz poprawki bezpieczeństwa.
Bądź zwięzły, konkretny i techniczny. Używaj języka polskiego.`

	userPrompt := fmt.Sprintf(`Zanalizuj poniższy zasób:
- ID: %s
- Dostawca: %s
- Typ: %s
- Miesięczny Koszt: $%.2f
- Czy publiczny: %v
- Tagi: %v

Wykryte PII w tym zasobie:
`, res.ResourceId, res.Provider, res.Type, res.EstimatedCost, res.IsPublic, res.Tags)

	for _, f := range findings {
		userPrompt += fmt.Sprintf("- Typ: %s, Ilość: %d
", f.PiiType, f.OccurrenceCount)
	}

	if len(findings) == 0 {
		userPrompt += "- Brak wykrytych PII.
"
	}

	fullPrompt := fmt.Sprintf("%s

%s", systemPrompt, userPrompt)
	
	return a.Ollama.Generate(a.Model, fullPrompt)
}
