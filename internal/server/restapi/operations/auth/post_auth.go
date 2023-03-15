// Code generated by go-swagger; DO NOT EDIT.

package auth

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the generate command

import (
	"net/http"

	"github.com/go-openapi/runtime/middleware"
)

// PostAuthHandlerFunc turns a function with the right signature into a post auth handler
type PostAuthHandlerFunc func(PostAuthParams, interface{}) middleware.Responder

// Handle executing the request and returning a response
func (fn PostAuthHandlerFunc) Handle(params PostAuthParams, principal interface{}) middleware.Responder {
	return fn(params, principal)
}

// PostAuthHandler interface for that can handle valid post auth params
type PostAuthHandler interface {
	Handle(PostAuthParams, interface{}) middleware.Responder
}

// NewPostAuth creates a new http.Handler for the post auth operation
func NewPostAuth(ctx *middleware.Context, handler PostAuthHandler) *PostAuth {
	return &PostAuth{Context: ctx, Handler: handler}
}

/*
	PostAuth swagger:route POST /auth Auth postAuth

Auth request
*/
type PostAuth struct {
	Context *middleware.Context
	Handler PostAuthHandler
}

func (o *PostAuth) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	route, rCtx, _ := o.Context.RouteInfo(r)
	if rCtx != nil {
		*r = *rCtx
	}
	var Params = NewPostAuthParams()
	uprinc, aCtx, err := o.Context.Authorize(r, route)
	if err != nil {
		o.Context.Respond(rw, r, route.Produces, route, err)
		return
	}
	if aCtx != nil {
		*r = *aCtx
	}
	var principal interface{}
	if uprinc != nil {
		principal = uprinc.(interface{}) // this is really a interface{}, I promise
	}

	if err := o.Context.BindValidRequest(r, route, &Params); err != nil { // bind params
		o.Context.Respond(rw, r, route.Produces, route, err)
		return
	}

	res := o.Handler.Handle(Params, principal) // actually handle the request
	o.Context.Respond(rw, r, route.Produces, route, res)

}
