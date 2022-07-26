package nginx

import (
	"bufio"
	"github.com/emirpasic/gods/stacks/linkedliststack"
	"github.com/pkg/errors"
	"os"
	"strings"
	"unicode"
)

const (
	Server       = "server"
	Location     = "location"
	Upstream     = "upstream"
	CommentStart = "#"
	Empty        = ""
)

func matchParentheses(stack *linkedliststack.Stack, v int32) {
	if v == '{' {
		stack.Push(v)
	} else if v == '}' {
		// stack is not empty and the top is == '{'
		if top, ok := stack.Peek(); ok && top == '{' {
			stack.Pop()
		} else {
			// fail
			stack.Push(v)
		}
	}
}

func parseDirective(scanner *bufio.Scanner) (d NgxDirective) {
	text := strings.TrimSpace(scanner.Text())
	// escape empty line or comment line
	if len(text) < 1 {
		return
	}

	if text[0] == '#' {
		d.Directive = "#"
		d.Params = strings.TrimLeft(text, "#")
		return
	}

	sep := len(text) - 1
	for k, v := range text {
		if unicode.IsSpace(v) {
			sep = k
			break
		}
	}

	d.Directive = text[0:sep]
	d.Params = text[sep:]

	stack := linkedliststack.New()

	if d.Directive == Server || d.Directive == Upstream || d.Directive == Location {
		// { } in one line
		// location = /.well-known/carddav { return 301 /remote.php/dav/; }
		if strings.Contains(d.Params, "{") {
			for _, v := range d.Params {
				matchParentheses(stack, v)
			}

			if stack.Empty() {
				return
			}
		}

		// location ^~ /.well-known {
		// location ^~ /.well-known
		// {
		// location ^~ /.well-known
		//
		//    {
		// { } not in one line
		for scanner.Scan() {
			text = strings.TrimSpace(scanner.Text())
			// escape empty line
			if text == "" {
				continue
			}
			d.Params += "\n" + scanner.Text()
			for _, v := range text {
				matchParentheses(stack, v)
				if stack.Empty() {
					break
				}
			}
			if stack.Empty() {
				break
			}
		}
	}
	d.Params = strings.TrimSpace(d.Params)
	return
}

func ParseNgxConfig(filename string) (c *NgxConfig, err error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, errors.Wrap(err, "error open file in ParseNgxConfig")
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	c = NewNgxConfig(filename)
	for scanner.Scan() {
		d := parseDirective(scanner)
		paramsScanner := bufio.NewScanner(strings.NewReader(d.Params))
		switch d.Directive {
		case Server:
			c.parseServer(paramsScanner)
		case Upstream:
			c.parseUpstream(paramsScanner)
		case CommentStart:
			c.commentQueue.Enqueue(d.Params)
		}
	}

	if err = scanner.Err(); err != nil {
		return nil, errors.Wrap(err, "error scanner in ParseNgxConfig")
	}

	return c, nil
}
