// Code generated by go-swagger; DO NOT EDIT.

package operations

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the generate command

import (
	"net/http"

	"github.com/go-openapi/runtime/middleware"
)

// GetUsersStateUIDHandlerFunc turns a function with the right signature into a get users state UID handler
type GetUsersStateUIDHandlerFunc func(GetUsersStateUIDParams) middleware.Responder

// Handle executing the request and returning a response
func (fn GetUsersStateUIDHandlerFunc) Handle(params GetUsersStateUIDParams) middleware.Responder {
	return fn(params)
}

// GetUsersStateUIDHandler interface for that can handle valid get users state UID params
type GetUsersStateUIDHandler interface {
	Handle(GetUsersStateUIDParams) middleware.Responder
}

// NewGetUsersStateUID creates a new http.Handler for the get users state UID operation
func NewGetUsersStateUID(ctx *middleware.Context, handler GetUsersStateUIDHandler) *GetUsersStateUID {
	return &GetUsersStateUID{Context: ctx, Handler: handler}
}

/* GetUsersStateUID swagger:route GET /users/state/{uid} getUsersStateUid

GetUsersStateUID get users state UID API

*/
type GetUsersStateUID struct {
	Context *middleware.Context
	Handler GetUsersStateUIDHandler
}

func (o *GetUsersStateUID) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	route, rCtx, _ := o.Context.RouteInfo(r)
	if rCtx != nil {
		*r = *rCtx
	}
	var Params = NewGetUsersStateUIDParams()
	if err := o.Context.BindValidRequest(r, route, &Params); err != nil { // bind params
		o.Context.Respond(rw, r, route.Produces, route, err)
		return
	}

	res := o.Handler.Handle(Params) // actually handle the request
	o.Context.Respond(rw, r, route.Produces, route, res)

}