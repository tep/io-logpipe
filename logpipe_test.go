package logpipe

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"golang.org/x/sync/errgroup"
)

const lorumIpsum = "testdata/lorumipsum.txt"

func testLines() ([]string, error) {
	f, err := os.Open(lorumIpsum)
	if err != nil {
		return nil, err
	}

	s := bufio.NewScanner(f)

	var lines []string
	for s.Scan() {
		if l := strings.TrimSpace(s.Text()); l != "" {
			lines = append(lines, l)
		}
	}

	return lines, s.Err()
}

func TestLogPipe(t *testing.T) {
	t.Parallel()

	lines, err := testLines()
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Lines of test log output: %d", len(lines))

	lp := &LogPipe{Tag: "test:stdout"}

	var got []string

	stdout := lp.MustAdd(func(msg string, args ...interface{}) {
		got = append(got, strings.TrimSpace(fmt.Sprintf(msg, args...)))
	})

	want := make([]string, len(lines))
	for i, l := range lines {
		want[i] = fmt.Sprintf("[test:stdout] %s", l)
	}

	eg, ctx := errgroup.WithContext(context.Background())
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	eg.Go(func() error {
		return lp.Run(ctx)
	})

	eg.Go(func() error {
		defer lp.Close()

		for i, l := range lines {
			t.Logf("--> %d) >>>%s<<<", i, l)
			if _, err := fmt.Fprintln(stdout, l); err != nil {
				return err
			}
		}

		return nil
	})

	if err := eg.Wait(); err != nil {
		t.Error(err)
	}

	for i, l := range got {
		t.Logf("<-- %d) >>>%s<<<", i, l)
		if l != want[i] {
			t.Errorf("log line %d differs:\n     Got: %q\n  Wanted: %q", i, l, want[i])
		}
	}
}
