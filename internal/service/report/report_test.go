package report

import (
	"reflect"
	"testing"
	"time"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	start := time.Date(2026, 3, 27, 10, 11, 12, 0, time.UTC)
	end := start.Add(3*time.Minute + 15*time.Second)

	in := &Report{
		StartedAt:    start,
		FinishedAt:   end,
		Duration:     "3m15s",
		InputDir:     "dataset",
		OutputDir:    "output",
		TotalImages:  120,
		TotalFaces:   340,
		TotalPersons: 22,
		Errors:       2,
		FileErrors: map[string]string{
			"img-01.jpg": "decode failed",
		},
		Threshold: 0.55,
		GPU:       true,
		Persons: []PersonReport{
			{
				ID:           1,
				PhotoCount:   12,
				FaceCount:    21,
				Thumbnail:    "Person_1/thumb.jpg",
				AvatarPath:   "avatars/Person_1.jpg",
				QualityScore: 12345.67,
				Photos:       []string{"Person_1/a.jpg", "Person_1/b.jpg"},
			},
		},
	}

	if err := Save(in, dir); err != nil {
		t.Fatalf("save report: %v", err)
	}

	out, err := Load(dir)
	if err != nil {
		t.Fatalf("load report: %v", err)
	}

	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round-trip mismatch:\nwant=%+v\ngot=%+v", in, out)
	}
}

func TestLoadMissingFile(t *testing.T) {
	t.Parallel()

	_, err := Load(t.TempDir())
	if err == nil {
		t.Fatal("expected error when report.json is missing, got nil")
	}
}
