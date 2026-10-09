package fake

import (
	"fmt"
	"maps"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"time"
)

const maxSpan = 2 * 365 * 24 * time.Hour

type generator func(l locale, r *rand.Rand, now time.Time) any

var generators = map[string]generator{
	"person.first_name": func(l locale, r *rand.Rand, _ time.Time) any { return pick(r, l.firstNames) },
	"person.last_name":  func(l locale, r *rand.Rand, _ time.Time) any { return pick(r, l.lastNames) },
	"person.name": func(l locale, r *rand.Rand, _ time.Time) any {
		return pick(r, l.firstNames) + " " + pick(r, l.lastNames)
	},
	"email": func(l locale, r *rand.Rand, _ time.Time) any {
		return strings.ToLower(pick(r, l.firstNames)+"."+pick(r, l.lastNames)) + "@example." + pick(r, []string{"com", "net", "org"})
	},
	"username": func(l locale, r *rand.Rand, _ time.Time) any {
		return strings.ToLower(pick(r, l.firstNames)) + pad(r.IntN(100), 2)
	},
	"phone":   func(l locale, r *rand.Rand, _ time.Time) any { return l.phone(r.IntN) },
	"city":    func(l locale, r *rand.Rand, _ time.Time) any { return pick(r, l.cities) },
	"country": func(l locale, r *rand.Rand, _ time.Time) any { return pick(r, l.countries) },
	"company": func(l locale, r *rand.Rand, _ time.Time) any { return pick(r, l.companies) },
	"word":    func(_ locale, r *rand.Rand, _ time.Time) any { return pick(r, words) },
	"sentence": func(_ locale, r *rand.Rand, _ time.Time) any {
		n := 6 + r.IntN(7)
		ws := make([]string, n)
		for i := range ws {
			ws[i] = pick(r, words)
		}
		ws[0] = strings.ToUpper(ws[0][:1]) + ws[0][1:]
		return strings.Join(ws, " ") + "."
	},
	"price": func(_ locale, r *rand.Rand, _ time.Time) any { return float64(100+r.IntN(49901)) / 100 },
	"image.url": func(_ locale, r *rand.Rand, _ time.Time) any {
		return fmt.Sprintf("https://picsum.photos/seed/%08x/640/480", r.Uint32())
	},
	"date.past": func(_ locale, r *rand.Rand, now time.Time) any {
		return now.Add(-span(r)).UTC().Truncate(time.Second).Format(time.RFC3339)
	},
	"date.future": func(_ locale, r *rand.Rand, now time.Time) any {
		return now.Add(span(r)).UTC().Truncate(time.Second).Format(time.RFC3339)
	},
}

func Names() []string { return slices.Sorted(maps.Keys(generators)) }

func Generate(name, localeName string, r *rand.Rand, now time.Time) (any, bool) {
	g, ok := generators[name]
	if !ok {
		return nil, false
	}
	l, ok := locales[localeName]
	if !ok {
		l = locales["en"]
	}
	return g(l, r, now), true
}

func pick(r *rand.Rand, list []string) string { return list[r.IntN(len(list))] }

func span(r *rand.Rand) time.Duration {
	return time.Hour + time.Duration(r.Int64N(int64(maxSpan-time.Hour)))
}

func pad(n, width int) string {
	s := strconv.Itoa(n)
	return strings.Repeat("0", max(0, width-len(s))) + s
}
