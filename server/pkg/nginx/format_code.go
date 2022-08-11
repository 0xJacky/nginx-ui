package nginx

import (
    "bufio"
    "github.com/emirpasic/gods/stacks/linkedliststack"
    "strings"
)

func fmtCode(content string) (fmtContent string) {
    fmtContent = fmtCodeWithIndent(content, 0)
    return
}

func fmtCodeWithIndent(content string, indent int) (fmtContent string) {
    /*
       Format content
       1. TrimSpace for each line
       2. use stack to count how many \t should add
    */
    stack := linkedliststack.New()

    scanner := bufio.NewScanner(strings.NewReader(content))

    for scanner.Scan() {
        text := scanner.Text()
        text = strings.TrimSpace(text)

        before := stack.Size()

        for _, char := range text {
            matchParentheses(stack, char)
        }

        after := stack.Size()

        fmtContent += strings.Repeat("\t", indent)

        if before == after {
            fmtContent += strings.Repeat("\t", stack.Size()) + text + "\n"
        } else {
            fmtContent += text + "\n"
        }

    }

    fmtContent = strings.Trim(fmtContent, "\n")

    return
}
