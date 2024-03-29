// Code generated by go-swagger; DO NOT EDIT.

package models

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"context"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
)

// UserStateResponse user state response
//
// swagger:model UserStateResponse
type UserStateResponse struct {

	// user state
	UserState *UserState `json:"user_state,omitempty"`
}

// Validate validates this user state response
func (m *UserStateResponse) Validate(formats strfmt.Registry) error {
	var res []error

	if err := m.validateUserState(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *UserStateResponse) validateUserState(formats strfmt.Registry) error {
	if swag.IsZero(m.UserState) { // not required
		return nil
	}

	if m.UserState != nil {
		if err := m.UserState.Validate(formats); err != nil {
			if ve, ok := err.(*errors.Validation); ok {
				return ve.ValidateName("user_state")
			} else if ce, ok := err.(*errors.CompositeError); ok {
				return ce.ValidateName("user_state")
			}
			return err
		}
	}

	return nil
}

// ContextValidate validate this user state response based on the context it is used
func (m *UserStateResponse) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	var res []error

	if err := m.contextValidateUserState(ctx, formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *UserStateResponse) contextValidateUserState(ctx context.Context, formats strfmt.Registry) error {

	if m.UserState != nil {
		if err := m.UserState.ContextValidate(ctx, formats); err != nil {
			if ve, ok := err.(*errors.Validation); ok {
				return ve.ValidateName("user_state")
			} else if ce, ok := err.(*errors.CompositeError); ok {
				return ce.ValidateName("user_state")
			}
			return err
		}
	}

	return nil
}

// MarshalBinary interface implementation
func (m *UserStateResponse) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *UserStateResponse) UnmarshalBinary(b []byte) error {
	var res UserStateResponse
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}
