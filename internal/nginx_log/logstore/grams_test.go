package logstore

import (
	"bytes"
	"context"
	"math/rand"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestGramFilterHasNoFalseNegatives checks that every substring of every
// line, in any letter case, passes the filter of its block.
func TestGramFilterHasNoFalseNegatives(t *testing.T) {
	var raw []byte
	var lines []string
	for _, r := range genRows(3000, 3) {
		lines = append(lines, r.Raw)
		raw = append(raw, r.Raw...)
		raw = append(raw, '\n')
	}
	f, ok := parseGramFilter(buildGramFilter(raw))
	require.True(t, ok)

	r := rand.New(rand.NewSource(5))
	for _, line := range lines {
		for range 5 {
			i := r.Intn(len(line))
			j := min(len(line), i+3+r.Intn(20))
			term := strings.ToUpper(line[i:j])
			require.True(t, f.mayContain(termGrams([]byte(lowerASCII(term)))), term)
		}
	}
}

func TestGramFilterRulesOutAbsentTerms(t *testing.T) {
	raw := []byte("GET /index.html 200\nPOST /api/login 401\n")
	f, ok := parseGramFilter(buildGramFilter(raw))
	require.True(t, ok)

	require.True(t, f.mayContain(termGrams([]byte("api/log"))))
	require.True(t, f.mayContain(termGrams([]byte("po"))), "a term shorter than a trigram always may occur")
	require.False(t, f.mayContain(termGrams([]byte("sqlmap"))))
	// A trigram across the end of a line is not in any line.
	require.False(t, f.mayContain(termGrams([]byte("200\npo"))))
}

func TestGramFilterSizeFollowsDistinctTrigrams(t *testing.T) {
	small, ok := parseGramFilter(buildGramFilter([]byte("aaaa\n")))
	require.True(t, ok)
	require.EqualValues(t, gramMinLog2, small.log2)

	var raw []byte
	r := rand.New(rand.NewSource(9))
	for range 20_000 {
		for range 40 {
			raw = append(raw, byte(' '+r.Intn(95)))
		}
		raw = append(raw, '\n')
	}
	large, ok := parseGramFilter(buildGramFilter(raw))
	require.True(t, ok)
	require.Greater(t, large.log2, small.log2)
	require.LessOrEqual(t, large.log2, uint8(gramMaxLog2))
}

func TestParseGramFilterRejectsMalformedPayloads(t *testing.T) {
	for _, p := range [][]byte{nil, {gramMinLog2}, {gramMaxLog2 + 1}, append([]byte{gramMinLog2}, make([]byte, 7)...)} {
		_, ok := parseGramFilter(p)
		require.False(t, ok, p)
	}
}

func TestLowerASCIIInPlaceMatchesBytewise(t *testing.T) {
	r := rand.New(rand.NewSource(11))
	for n := range 64 {
		b := make([]byte, n)
		for i := range b {
			b[i] = byte(r.Intn(256))
		}
		want := bytes.Clone(b)
		for i := range want {
			want[i] = lowerByte(want[i])
		}
		lowerASCIIInPlace(b)
		require.Equal(t, want, b)
	}
}

// TestTextSearchSkipsBlocksTheFilterRulesOut checks that an absent term does
// not decompress any raw text, and that blocks written without a filter, by
// an earlier version, still answer the same.
func TestTextSearchSkipsBlocksTheFilterRulesOut(t *testing.T) {
	rows := genRows(4000, 13)
	s := openTestStore(t, t.TempDir(), 500)
	ingest(t, s, rows, 0)
	ctx := context.Background()

	cases := []struct{ terms, excluded []string }{
		{terms: []string{"zzz"}},
		{terms: []string{"Chrome"}},
		{terms: []string{"10.0.1."}},
		{terms: []string{`"HTTP/1.1" 404`}},
		{terms: []string{"post"}, excluded: []string{"curl"}},
		{terms: []string{"get"}, excluded: []string{"zzz"}},
		{excluded: []string{"mozilla"}},
	}
	queries := make([]*Query, len(cases))
	want := make([]uint64, len(cases))
	for i, c := range cases {
		q := &Query{}
		expected := 0
		for _, term := range c.terms {
			q.Clauses = append(q.Clauses, Clause{Kind: ClauseText, Text: term})
		}
		for _, term := range c.excluded {
			q.Clauses = append(q.Clauses, Clause{Negate: true, Kind: ClauseText, Text: term})
		}
		for _, r := range rows {
			if containsAll(r.Raw, c.terms) && !containsAny(r.Raw, c.excluded) {
				expected++
			}
		}
		n, err := s.Count(ctx, q)
		require.NoError(t, err)
		require.EqualValues(t, expected, n, c)
		queries[i], want[i] = q, n
	}

	// The absent term is settled by the filters alone: the raw payload of no
	// block can be read when every slot is taken.
	for range cap(s.textSlots) {
		s.textSlots <- struct{}{}
	}
	n, err := s.Count(ctx, &Query{Text: "zzz"})
	require.NoError(t, err)
	require.Zero(t, n)
	for range cap(s.textSlots) {
		<-s.textSlots
	}

	// Without filters every block is scanned and the answers stay the same.
	dropGramFilters(s)
	for i, q := range queries {
		n, err := s.Count(ctx, q)
		require.NoError(t, err)
		require.Equal(t, want[i], n, q)
	}
}

func TestTextTermWithNewlineNeverMatches(t *testing.T) {
	rows := genRows(500, 17)
	s := openTestStore(t, t.TempDir(), 100)
	ingest(t, s, rows, 0)
	dropGramFilters(s)
	n, err := s.Count(context.Background(), &Query{Clauses: []Clause{{Kind: ClauseText, Text: "1.1\" 200"}}})
	require.NoError(t, err)
	require.NotZero(t, n)
	n, err = s.Count(context.Background(), &Query{Clauses: []Clause{{Kind: ClauseText, Text: "\n10.0"}}})
	require.NoError(t, err)
	require.Zero(t, n)
}

// dropGramFilters makes the store's blocks look like ones written before gram
// filters existed. No query may run meanwhile.
func dropGramFilters(s *Store) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, seg := range s.segments {
		if st := seg.state.Load(); st != nil {
			for _, m := range st.blocks {
				m.index[ColRaw][payloadGrams] = -1
			}
		}
	}
}

func containsAll(line string, terms []string) bool {
	for _, t := range terms {
		if !strings.Contains(strings.ToLower(line), strings.ToLower(t)) {
			return false
		}
	}
	return true
}

func containsAny(line string, terms []string) bool {
	for _, t := range terms {
		if strings.Contains(strings.ToLower(line), strings.ToLower(t)) {
			return true
		}
	}
	return false
}
