package fake_test

import (
	"math"
	"math/rand/v2"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/demola234/tiny-tools/mock_machina/internal/fake"
)

var now = time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)

func gen(t *testing.T, name, locale string, seed uint64) any {
	t.Helper()
	v, ok := fake.Generate(name, locale, rand.New(rand.NewPCG(seed, 9)), now)
	if !ok {
		t.Fatalf("Generate(%q) isn't a generator", name)
	}
	return v
}

func TestNames(t *testing.T) {
	t.Parallel()

	want := []string{
		"company", "city", "country", "date.future", "date.past", "email", "image.url",
		"person.first_name", "person.last_name", "person.name", "phone", "price", "sentence", "username", "word",
	}
	got := fake.Names()
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("Names() = %v\nwant %v", got, want)
	}
}

func TestGenerate_EveryGeneratorEveryLocale(t *testing.T) {
	t.Parallel()

	for _, locale := range []string{"en", "en_NG", ""} {
		for _, name := range fake.Names() {
			a, b := gen(t, name, locale, 1), gen(t, name, locale, 1)
			if a != b {
				t.Errorf("%s/%s isn't deterministic: %v, %v", locale, name, a, b)
			}
			if s, ok := a.(string); ok && strings.TrimSpace(s) == "" {
				t.Errorf("%s/%s is empty", locale, name)
			}
		}
	}
	if _, ok := fake.Generate("person.nam", "en", rand.New(rand.NewPCG(1, 1)), now); ok {
		t.Error("an unknown generator was accepted")
	}
}

func TestGenerate_Varies(t *testing.T) {
	t.Parallel()

	seen := map[any]bool{}
	for s := range uint64(50) {
		seen[gen(t, "person.name", "en", s)] = true
	}
	if len(seen) < 20 {
		t.Errorf("50 seeds gave only %d names", len(seen))
	}
}

func TestGenerate_Formats(t *testing.T) {
	t.Parallel()

	patterns := map[string]*regexp.Regexp{
		"email":     regexp.MustCompile(`^[a-z]+\.[a-z]+@example\.(com|net|org)$`),
		"username":  regexp.MustCompile(`^[a-z]+\d{2}$`),
		"image.url": regexp.MustCompile(`^https://picsum\.photos/seed/[0-9a-f]{8}/640/480$`),
		"sentence":  regexp.MustCompile(`^[A-Z][a-z]+( [a-z]+){5,11}\.$`),
		"word":      regexp.MustCompile(`^[a-z]+$`),
	}
	for name, re := range patterns {
		for s := range uint64(20) {
			if v, _ := gen(t, name, "en", s).(string); !re.MatchString(v) {
				t.Errorf("%s = %q, doesn't match %s", name, v, re)
			}
		}
	}
}

func TestGenerate_Values(t *testing.T) {
	t.Parallel()

	for s := range uint64(20) {
		if v, _ := gen(t, "phone", "en_NG", s).(string); !regexp.MustCompile(`^\+234 [789]0\d \d{3} \d{4}$`).MatchString(v) {
			t.Errorf("en_NG phone = %q", v)
		}
		if v, _ := gen(t, "phone", "en", s).(string); !regexp.MustCompile(`^\+1 \d{3}-555-01\d{2}$`).MatchString(v) {
			t.Errorf("en phone = %q", v)
		}
		price, _ := gen(t, "price", "en", s).(float64)
		if price < 1 || price > 500 || math.Abs(price*100-math.Round(price*100)) > 1e-6 {
			t.Errorf("price = %v", price)
		}
		past, _ := time.Parse(time.RFC3339, gen(t, "date.past", "en", s).(string))
		future, _ := time.Parse(time.RFC3339, gen(t, "date.future", "en", s).(string))
		if !past.Before(now) || past.Before(now.AddDate(-2, 0, 0)) || !future.After(now) || future.After(now.AddDate(2, 0, 0)) {
			t.Errorf("dates: past %v, future %v", past, future)
		}
	}
	if got := gen(t, "country", "en_NG", 1); got != "Nigeria" {
		t.Errorf("en_NG country = %v", got)
	}
}

func TestGenerate_LocaleNames(t *testing.T) {
	t.Parallel()

	ng := map[any]bool{}
	en := map[any]bool{}
	for s := range uint64(40) {
		ng[gen(t, "person.last_name", "en_NG", s)] = true
		en[gen(t, "person.last_name", "en", s)] = true
	}
	for name := range ng {
		if en[name] {
			t.Errorf("%v is in both locales' last names", name)
		}
	}
}
