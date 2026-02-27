package hclconfig

import (
	"fmt"
	"os"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

// newBaseEvalContext creates an EvalContext with the built-in env() and env_or()
// functions and merges any user-supplied context. When strictEnv is true, env()
// returns an error for undefined environment variables.
func newBaseEvalContext(userCtx *hcl.EvalContext, strictEnv bool) *hcl.EvalContext {
	ctx := &hcl.EvalContext{
		Variables: make(map[string]cty.Value),
		Functions: map[string]function.Function{
			"env":    envFunction(strictEnv),
			"env_or": envOrFunction(),
		},
	}

	if userCtx != nil {
		for k, v := range userCtx.Variables {
			ctx.Variables[k] = v
		}
		for k, v := range userCtx.Functions {
			ctx.Functions[k] = v
		}
	}

	return ctx
}

func envFunction(strict bool) function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "name",
				Type: cty.String,
			},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			name := args[0].AsString()
			val, ok := os.LookupEnv(name)
			if !ok && strict {
				return cty.NilVal, fmt.Errorf("environment variable %q is not set", name)
			}
			return cty.StringVal(val), nil
		},
	})
}

func envOrFunction() function.Function {
	return function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name: "name",
				Type: cty.String,
			},
			{
				Name: "default",
				Type: cty.String,
			},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			name := args[0].AsString()
			val, ok := os.LookupEnv(name)
			if !ok {
				return args[1], nil
			}
			return cty.StringVal(val), nil
		},
	})
}
