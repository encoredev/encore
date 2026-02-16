package natspubsub

import "testing"

func TestDefaultStreamName(t *testing.T) {
	got := defaultStreamName("orders.created.*")
	if got == "" {
		t.Fatal("defaultStreamName returned empty value")
	}
	if got == "encore_pubsub_orders.created.*" {
		t.Fatalf("stream name %q was not sanitized", got)
	}
}

func TestEnsureWildcardCoverage(t *testing.T) {
	topic := &Topic[int]{subject: "orders", streamSubjects: []string{"orders"}}
	topic.ensureWildcardCoverage("orders")

	found := false
	for _, s := range topic.streamSubjects {
		if s == "orders.>" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected wildcard stream subject to be added, got %v", topic.streamSubjects)
	}
}

func TestSubjectsCover(t *testing.T) {
	if !subjectsCover([]string{"orders.>"}, []string{"orders.1", "orders.2"}) {
		t.Fatal("expected wildcard subject to cover required subjects")
	}
	if subjectsCover([]string{"orders.created"}, []string{"orders.updated"}) {
		t.Fatal("did not expect unrelated subjects to collide")
	}
}
