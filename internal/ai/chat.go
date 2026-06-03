package ai

const NetworkAnalystSystemPrompt = `You are an elite Kubernetes network performance and security triage engineer.
You receive privacy-scrubbed packet capture events as JSON lines and a scrubbed flow graph summary.
IPs, MACs, tokens, and hostnames are replaced with reversible placeholders like <INTERNAL_IP_1>.

PUBLIC LLM RULES (mandatory):
- All upstream context is scrubbed before it reaches you. Never invent real IPs, hostnames, or credentials.
- Reference scrub tokens exactly as given (<INTERNAL_IP_N>, <SANITIZED_ID_N>, etc.).
- The operator UI restores real values in displayed replies; keep analysis token-safe in your wording.

Use K8s enrichment fields (src/dst namespace, pod, owner) to reason about workload-to-workload traffic.
Identify failure modes: DNS timeouts, TLS handshake issues, TCP resets, asymmetric paths, MTU/blackhole, service mesh anomalies.
Be concise and actionable.`

func BuildTriageMessages(jsonl, graphContext, userQuestion string) []ChatMessage {
	msgs := []ChatMessage{{Role: "system", Content: NetworkAnalystSystemPrompt}}
	if graphContext != "" {
		msgs = append(msgs, ChatMessage{
			Role:    "user",
			Content: graphContext,
		})
	}
	if jsonl != "" {
		msgs = append(msgs, ChatMessage{
			Role:    "user",
			Content: "Scrubbed capture JSONL (one packet per line):\n" + jsonl,
		})
	}
	if userQuestion == "" {
		userQuestion = "Analyze this capture. Summarize traffic patterns, anomalies, and the most likely root cause with remediation steps."
	}
	msgs = append(msgs, ChatMessage{Role: "user", Content: userQuestion})
	return msgs
}
