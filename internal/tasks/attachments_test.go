package tasks

import "testing"

func TestValidateAttachment(t *testing.T) {
	t.Parallel()

	if err := ValidateAttachment("report.pdf", "application/pdf", 1024, 10*1024*1024); err != nil {
		t.Fatalf("expected valid attachment, got %v", err)
	}

	if err := ValidateAttachment("report.pdf", "application/zip", 1024, 10*1024*1024); err == nil {
		t.Fatal("expected disallowed mime type error")
	}

	if err := ValidateAttachment("big.pdf", "application/pdf", 20*1024*1024, 10*1024*1024); err == nil {
		t.Fatal("expected size limit error")
	}
}
