// Copyright © 2018 Timothy E. Peoples <eng@toolman.org>
//
// This program is free software; you can redistribute it and/or
// modify it under the terms of the GNU General Public License
// as published by the Free Software Foundation; either version 2
// of the License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

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
