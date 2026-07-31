package gen_test

// The package next door is generated, so nothing in it can be reviewed for
// correctness — only the shape of what the generator was asked to produce can.
// This file asserts that shape. It is the executable half of api/codegen-server.yaml.

import (
	"context"
	"net/http"
	"reflect"
	"testing"

	"github.com/bryanster/purpleops/internal/httpapi/gen"
)

// Dropping either generator flag from api/codegen-server.yaml stops this file
// compiling: HandlerFromMux comes from `chi-server`, NewStrictHandler from
// `strict-server`, and M0B-006 mounts the API by calling both.
var (
	_ = gen.HandlerFromMux
	_ = gen.NewStrictHandler
)

// TestStrictServerInterfaceHandlersAreTyped is the acceptance criterion of
// M0B-005 in test form: handlers take a typed request and return a typed
// response. A handler that could reach the http.ResponseWriter could write a
// body the spec does not describe — which is drift, and drift is the thing the
// spec-first rule exists to prevent.
func TestStrictServerInterfaceHandlersAreTyped(t *testing.T) {
	var (
		iface          = reflect.TypeOf((*gen.StrictServerInterface)(nil)).Elem()
		contextType    = reflect.TypeOf((*context.Context)(nil)).Elem()
		errorType      = reflect.TypeOf((*error)(nil)).Elem()
		responseWriter = reflect.TypeOf((*http.ResponseWriter)(nil)).Elem()
		httpRequest    = reflect.TypeOf((*http.Request)(nil))
	)

	if iface.NumMethod() == 0 {
		t.Fatal("StrictServerInterface has no methods; the spec has operations, so the generator did not run over the document it should have")
	}

	for i := range iface.NumMethod() {
		method := iface.Method(i)

		t.Run(method.Name, func(t *testing.T) {
			signature := method.Type

			for j := range signature.NumIn() {
				switch signature.In(j) {
				case responseWriter:
					t.Errorf("takes an http.ResponseWriter; a strict handler returns a response object instead, so it cannot write anything the spec does not describe")
				case httpRequest:
					t.Errorf("takes an *http.Request; a strict handler receives a typed request object with the parameters already bound")
				}
			}

			if signature.NumIn() != 2 || signature.In(0) != contextType {
				t.Fatalf("signature takes %d arguments, want (context.Context, <request struct>)", signature.NumIn())
			}
			if kind := signature.In(1).Kind(); kind != reflect.Struct {
				t.Errorf("the request argument is a %s, want a generated request struct", kind)
			}

			if signature.NumOut() != 2 || signature.Out(1) != errorType {
				t.Fatalf("signature returns %d values, want (<response object>, error)", signature.NumOut())
			}
			if kind := signature.Out(0).Kind(); kind != reflect.Interface {
				t.Errorf("the response value is a %s, want the generated response interface — one implementation per documented status code", kind)
			}
		})
	}
}
