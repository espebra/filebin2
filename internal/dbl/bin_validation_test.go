package dbl

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/espebra/filebin2/internal/ds"
)

func TestGenerateId(t *testing.T) {
	d := &BinDao{} // No DB needed for ID generation

	// Test 1: Verify ID length
	id := d.GenerateId()
	if len(id) != 16 {
		t.Errorf("GenerateId() returned ID with length %d, want 16", len(id))
	}

	// Test 2: Verify ID contains only valid characters (a-z, 0-9)
	validPattern := regexp.MustCompile("^[a-z0-9]+$")
	if !validPattern.MatchString(id) {
		t.Errorf("GenerateId() returned ID with invalid characters: %q", id)
	}

	// Test 3: Verify multiple calls produce different IDs (randomness check)
	// Generate 100 IDs and ensure we get at least 95 unique values
	// (statistically, all 100 should be unique, but allowing small margin)
	ids := make(map[string]bool)
	iterations := 100
	for i := 0; i < iterations; i++ {
		generatedId := d.GenerateId()

		// Verify each ID has correct length and characters
		if len(generatedId) != 16 {
			t.Errorf("GenerateId() iteration %d: returned ID with length %d, want 16", i, len(generatedId))
		}
		if !validPattern.MatchString(generatedId) {
			t.Errorf("GenerateId() iteration %d: returned ID with invalid characters: %q", i, generatedId)
		}

		ids[generatedId] = true
	}

	uniqueCount := len(ids)
	minExpectedUnique := 95 // Allow for very small chance of collision
	if uniqueCount < minExpectedUnique {
		t.Errorf("GenerateId() generated only %d unique IDs out of %d iterations, want at least %d",
			uniqueCount, iterations, minExpectedUnique)
	}
}

func TestGenerateIdCharacterDistribution(t *testing.T) {
	d := &BinDao{}

	// Generate multiple IDs and verify we see both letters and numbers
	// This ensures the character set is being used properly
	hasLetters := false
	hasNumbers := false

	for i := 0; i < 10; i++ {
		id := d.GenerateId()
		for _, char := range id {
			if char >= 'a' && char <= 'z' {
				hasLetters = true
			}
			if char >= '0' && char <= '9' {
				hasNumbers = true
			}
		}
		if hasLetters && hasNumbers {
			break
		}
	}

	if !hasLetters {
		t.Error("GenerateId() did not generate any letters in 10 attempts")
	}
	if !hasNumbers {
		t.Error("GenerateId() did not generate any numbers in 10 attempts")
	}
}

func FuzzBinValidateInput(f *testing.F) {
	// Seed corpus with interesting inputs
	f.Add("abcdefgh")
	f.Add("")
	f.Add("short")
	f.Add(strings.Repeat("a", 61))
	f.Add(strings.Repeat("a", 60))
	f.Add(".abcdefgh")
	f.Add("valid-bin_name.1")
	f.Add("has spaces!!")
	f.Add("UPPERCASE123")
	f.Add("bin@#$%^&*")
	f.Add("\x00\x00\x00\x00\x00\x00\x00\x00")
	f.Add("café-latte1")
	f.Add("abcdefghijklmnop")

	d := &BinDao{}

	f.Fuzz(func(t *testing.T, input string) {
		bin := &ds.Bin{
			Id:        input,
			ExpiredAt: time.Now().Add(time.Hour),
		}
		err := d.ValidateInput(bin)

		if err != nil {
			return
		}

		// If validation passed, verify the invariants hold

		// Length must be between 8 and 60
		if len(input) < 8 {
			t.Errorf("ValidateInput accepted too short bin ID: %q (len=%d)", input, len(input))
		}
		if len(input) > 60 {
			t.Errorf("ValidateInput accepted too long bin ID: %q (len=%d)", input, len(input))
		}

		// Must not start with dot
		if strings.HasPrefix(input, ".") {
			t.Errorf("ValidateInput accepted bin ID starting with dot: %q", input)
		}

		// Must contain only valid characters [A-Za-z0-9-_.]
		validBin := regexp.MustCompile("^[A-Za-z0-9._-]+$")
		if !validBin.MatchString(input) {
			t.Errorf("ValidateInput accepted bin ID with invalid characters: %q", input)
		}
	})
}
