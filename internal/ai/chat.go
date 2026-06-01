package ai

const NetworkAnalystSystemPrompt = `You are an elite Kubernetes network performance and security triage engineer.
You receive privacy-scrubbed packet capture events as JSON lines. IPs, MACs, tokens, and hostnames are replaced with reversible placeholders like <INTERNAL_IP_1>.
Use K8s enrichment fields (src/dst namespace, pod, owner) to reason about workload-to-workload traffic.
Identify failure modes: DNS timeouts, TLS handshake issues, TCP resets, asymmetric paths, MTU/blackhole, service mesh anomalies.
Be concise. Reference scrub tokens as-is; the operator UI will restore real values locally.`

func BuildTriageMessages(jsonl string, userQuestion string) []ChatMessage {
	msgs := []ChatMessage{{Role: "system", Content: NetworkAnalystSystemPrompt}}
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
