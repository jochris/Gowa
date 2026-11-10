package owner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"

	"gowa/internal/command"
)

func init() {
	command.Register(&command.Command{
		Name:    "eval",
		Cmd:     "eval",
		Categ:   "owner",
		Desc:    "Evaluasi kode Go in-process dengan akses ctx (Khusus Owner)",
		Alias:   nil,
		Execute: handleYaegiEval,
	})
}

const yaegiTimeout = 30 * time.Second

// Prelude sengaja satu baris supaya nomor baris error yaegi sama dengan baris snippet.
const yaegiPrelude = "package evalrun; " +
	`import ("bytes"; "context"; "encoding/json"; "errors"; "fmt"; "io"; "math"; "net/http"; "os"; "regexp"; "sort"; "strconv"; "strings"; "time"; "evalenv"; "gowa/internal/command"); ` +
	`var (_ = bytes.NewBuffer; _ = context.Background; _ = json.Marshal; _ = errors.New; _ = fmt.Sprint; _ = io.EOF; _ = math.Pi; _ = http.StatusOK; _ = os.Getenv; _ = regexp.MustCompile; _ = sort.Ints; _ = strconv.Itoa; _ = strings.TrimSpace; _ = time.Now; _ = evalenv.Ctx; _ = (*command.Context)(nil)); ` +
	`func run() (_r interface{}) { ctx := evalenv.Ctx; _ = ctx; `

// Isi dengan symbol tambahan (struct/fungsi bot) supaya bisa dipakai dari snippet.
var yaegiExtraSymbols = interp.Exports{}

type yaegiOutput struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (o *yaegiOutput) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.buf.Write(p)
}

func (o *yaegiOutput) String() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.buf.String()
}

func handleYaegiEval(ctx *command.Context) error {
	if !ctx.IsOwner {
		return nil
	}

	code := stripCodeFence(ctx.RawArgs)
	if code == "" {
		return ctx.Reply("*Usage:* `eval <kode go>`\n\n_Variabel_ `ctx` _tersedia. Pakai_ `return <nilai>` _untuk menampilkan hasil._")
	}

	ctx.Typing()

	baseCtx := ctx.Ctx
	if baseCtx == nil {
		baseCtx = context.Background()
	}

	runCtx, cancel := context.WithTimeout(baseCtx, yaegiTimeout)
	defer cancel()

	out := &yaegiOutput{}
	start := time.Now()
	res, err := runYaegi(runCtx, ctx, code, out)
	elapsed := time.Since(start).Round(time.Millisecond)

	text := strings.TrimSpace(out.String())
	if res != nil {
		if e, ok := res.(error); ok {
			text = yaegiJoin(text, "error: "+e.Error())
		} else {
			text = yaegiJoin(text, fmt.Sprintf("%+v", res))
		}
	}

	switch {
	case runCtx.Err() == context.DeadlineExceeded:
		return ctx.Reply(formatEvalOutput(fmt.Sprintf("*Timeout* (%s):", yaegiTimeout), text))
	case err != nil:
		return ctx.Reply(formatEvalOutput("*Error:*", yaegiJoin(text, err.Error())))
	case text == "":
		return nil
	}

	return ctx.Reply(formatEvalOutput(fmt.Sprintf("_%s_", elapsed), text))
}

func runYaegi(parent context.Context, botCtx *command.Context, code string, out io.Writer) (res interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()

	i := interp.New(interp.Options{Stdout: out, Stderr: out})

	if err := i.Use(stdlib.Symbols); err != nil {
		return nil, err
	}

	holder := botCtx
	bridge := interp.Exports{
		"gowa/internal/command/command": {
			"Context": reflect.ValueOf((*command.Context)(nil)),
		},
		"evalenv/evalenv": {
			"Ctx": reflect.ValueOf(&holder).Elem(),
		},
	}
	if err := i.Use(bridge); err != nil {
		return nil, err
	}
	if len(yaegiExtraSymbols) > 0 {
		if err := i.Use(yaegiExtraSymbols); err != nil {
			return nil, err
		}
	}

	if _, err := i.Eval(yaegiPrelude + code + "\nreturn nil\n}\n"); err != nil {
		return nil, err
	}

	v, err := i.EvalWithContext(parent, "evalrun.run()")
	if err != nil {
		return nil, err
	}
	if v.IsValid() && v.CanInterface() {
		res = v.Interface()
	}
	return res, nil
}

func yaegiJoin(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	}
	return a + "\n" + b
}

func stripCodeFence(s string) string {
	s = strings.TrimSpace(s)

	if !strings.HasPrefix(s, "```") {
		return s
	}

	s = strings.TrimPrefix(s, "```")

	if i := strings.IndexByte(s, '\n'); i >= 0 &&
		!strings.ContainsAny(s[:i], " \t;{}()") {
		s = s[i+1:]
	}

	return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "```"))
}

func formatEvalOutput(header, body string) string {
	body = strings.ToValidUTF8(body, "")

	const maxOutput = 4000
	truncated := len(body) > maxOutput

	if truncated {
		body = strings.ToValidUTF8(body[:maxOutput], "")
	}

	if body == "" {
		return header
	}

	result := fmt.Sprintf("%s\n```\n%s\n```", header, body)

	if truncated {
		result += "\n_... (output dipotong)_"
	}

	return result
}
