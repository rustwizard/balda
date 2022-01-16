// Code generated by go-swagger; DO NOT EDIT.

package application

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"net/http"

	"github.com/go-openapi/runtime"

	"balda/models"
)

// PostSignupOKCode is the HTTP code returned for type PostSignupOK
const PostSignupOKCode int = 200

/*PostSignupOK Response for signup request

swagger:response postSignupOK
*/
type PostSignupOK struct {

	/*
	  In: Body
	*/
	Payload *models.SignupResponse `json:"body,omitempty"`
}

// NewPostSignupOK creates PostSignupOK with default headers values
func NewPostSignupOK() *PostSignupOK {

	return &PostSignupOK{}
}

// WithPayload adds the payload to the post signup o k response
func (o *PostSignupOK) WithPayload(payload *models.SignupResponse) *PostSignupOK {
	o.Payload = payload
	return o
}

// SetPayload sets the payload to the post signup o k response
func (o *PostSignupOK) SetPayload(payload *models.SignupResponse) {
	o.Payload = payload
}

// WriteResponse to the client
func (o *PostSignupOK) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {

	rw.WriteHeader(200)
	if o.Payload != nil {
		payload := o.Payload
		if err := producer.Produce(rw, payload); err != nil {
			panic(err) // let the recovery middleware deal with this
		}
	}
}

// PostSignupBadRequestCode is the HTTP code returned for type PostSignupBadRequest
const PostSignupBadRequestCode int = 400

/*PostSignupBadRequest Error when signup

swagger:response postSignupBadRequest
*/
type PostSignupBadRequest struct {

	/*
	  In: Body
	*/
	Payload *models.ErrorResponse `json:"body,omitempty"`
}

// NewPostSignupBadRequest creates PostSignupBadRequest with default headers values
func NewPostSignupBadRequest() *PostSignupBadRequest {

	return &PostSignupBadRequest{}
}

// WithPayload adds the payload to the post signup bad request response
func (o *PostSignupBadRequest) WithPayload(payload *models.ErrorResponse) *PostSignupBadRequest {
	o.Payload = payload
	return o
}

// SetPayload sets the payload to the post signup bad request response
func (o *PostSignupBadRequest) SetPayload(payload *models.ErrorResponse) {
	o.Payload = payload
}

// WriteResponse to the client
func (o *PostSignupBadRequest) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {

	rw.WriteHeader(400)
	if o.Payload != nil {
		payload := o.Payload
		if err := producer.Produce(rw, payload); err != nil {
			panic(err) // let the recovery middleware deal with this
		}
	}
}
