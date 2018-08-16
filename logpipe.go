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
// ---------------------------------------------------------------------------

// Package logpipe provides a simple facility for interleaving child process
// output into your existing logs.
// TODO(tep): Add much better documentation XXX
package logpipe // import "toolman.org/io/logpipe"

import (
	"bufio"
	"context"
	"io"
	"os"
	"sync"

	"golang.org/x/sync/errgroup"
)

// LogPipe is the dispatcher for routing disparate output streams. It's zero
// value may be used directly or you may use a pointer reference.
type LogPipe struct {
	Tag     string
	streams []*logStream
}

func (lp *LogPipe) tag() string {
	if lp.Tag != "" {
		return lp.Tag
	}
	return "logpipe"
}

func (lp *LogPipe) MustAdd(logFunc LogFunc) io.Writer {
	return lp.MustAddTagged(lp.tag(), logFunc)
}

func (lp *LogPipe) MustAddTagged(tag string, logFunc LogFunc) io.Writer {
	w, err := lp.AddTagged(tag, logFunc)
	if err != nil {
		panic(err)
	}
	return w
}

func (lp *LogPipe) Add(logFunc LogFunc) (io.Writer, error) {
	return lp.AddTagged(lp.tag(), logFunc)
}

func (lp *LogPipe) AddTagged(tag string, logFunc LogFunc) (io.Writer, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	ls := &logStream{
		tag:    tag,
		reader: r,
		writer: w,
		logger: logFunc,
		done:   make(chan struct{}),
	}

	lp.streams = append(lp.streams, ls)

	return w, nil
}

func (lp *LogPipe) Run(ctx context.Context) error {
	eg, ctx := errgroup.WithContext(ctx)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for _, ls := range lp.streams {
		func(ls *logStream) {
			eg.Go(func() error {
				return ls.run(ctx)
			})
		}(ls)
	}

	return eg.Wait()
}

func (lp *LogPipe) Close() {
	for _, ls := range lp.streams {
		ls.close()
	}
}

type LogFunc func(string, ...interface{})

type logStream struct {
	tag    string
	reader io.ReadCloser
	writer io.WriteCloser
	logger LogFunc
	done   chan struct{}
	sync.Once
}

func (ls *logStream) close() {
	ls.Do(func() {
		close(ls.done)
		ls.reader.Close()
		ls.writer.Close()
	})
}

func (ls *logStream) run(ctx context.Context) error {
	s := bufio.NewScanner(ls.reader)

	for {
		select {
		case <-ls.done:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if !s.Scan() {
			break
		}

		ls.logger("[%s] %s\n", ls.tag, s.Text())
	}

	if err := s.Err(); err != nil && !isPipeAlreadyClosed(err) {
		return err
	}

	return nil
}

func isPipeAlreadyClosed(err error) bool {
	if perr, ok := err.(*os.PathError); ok && perr.Op == "read" && perr.Path == "|0" && perr.Err.Error() == "file already closed" {
		return true
	}
	return false
}
