// Code generated by go-swagger; DO NOT EDIT.

package restapi

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"encoding/json"
)

var (
	// SwaggerJSON embedded version of the swagger document used at generation time
	SwaggerJSON json.RawMessage
	// FlatSwaggerJSON embedded flattened version of the swagger document used at generation time
	FlatSwaggerJSON json.RawMessage
)

func init() {
	SwaggerJSON = json.RawMessage([]byte(`{
  "schemes": [
    "http"
  ],
  "swagger": "2.0",
  "info": {
    "description": "Balda GameServer API methods and models\u003cbr\u003e \u003ch3\u003eHeaders\u003c/h3\u003e\u003cbr/\u003e \u003ctable\u003e \u003ctr\u003e\u003ctd\u003e\u003cb\u003e\u003ci\u003eX-API-Key\u003c/i\u003e\u003c/b\u003e\u003c/td\u003e\u003ctd\u003eAPI key is a special token that the client needs to provide when making API calls.\u003c/td\u003e\u003c/tr\u003e \u003ctr\u003e\u003ctd\u003e\u003cb\u003e\u003ci\u003eX-API-User\u003c/i\u003e\u003c/b\u003e\u003c/td\u003e\u003ctd\u003eUser's ID\u003c/td\u003e\u003c/tr\u003e \u003ctr\u003e\u003ctd\u003e\u003cb\u003e\u003ci\u003eX-API-Session\u003c/i\u003e\u003c/b\u003e\u003c/td\u003e\u003ctd\u003eSession ID\u003c/td\u003e\u003c/tr\u003e \u003c/table\u003e",
    "title": "Balda GameServer",
    "version": "1.0.0"
  },
  "host": "127.0.0.1:9666",
  "basePath": "/balda/api/v1",
  "paths": {
    "/auth": {
      "post": {
        "security": [
          {
            "APIKeyHeader": []
          }
        ],
        "consumes": [
          "application/json"
        ],
        "tags": [
          "Application"
        ],
        "summary": "Auth request",
        "parameters": [
          {
            "name": "body",
            "in": "body",
            "schema": {
              "$ref": "#/definitions/AuthRequest"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "Response for auth request",
            "schema": {
              "$ref": "#/definitions/AuthResponse"
            }
          },
          "401": {
            "description": "Error when auth",
            "schema": {
              "$ref": "#/definitions/ErrorResponse"
            }
          }
        }
      }
    },
    "/signup": {
      "post": {
        "consumes": [
          "application/json"
        ],
        "tags": [
          "Application"
        ],
        "summary": "Sign-up request",
        "parameters": [
          {
            "name": "body",
            "in": "body",
            "schema": {
              "$ref": "#/definitions/SignupRequest"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "Response for signup request",
            "schema": {
              "$ref": "#/definitions/SignupResponse"
            }
          },
          "400": {
            "description": "Error when signup",
            "schema": {
              "$ref": "#/definitions/ErrorResponse"
            }
          }
        }
      }
    }
  },
  "definitions": {
    "AuthRequest": {
      "properties": {
        "email": {
          "description": "User's email",
          "type": "string",
          "example": "js@example.org"
        },
        "firstname": {
          "description": "User's first name",
          "type": "string",
          "example": "John"
        },
        "lastname": {
          "description": "User's last name",
          "type": "string",
          "example": "Smith"
        },
        "password": {
          "description": "User's password",
          "type": "string",
          "example": "some_password"
        }
      }
    },
    "AuthResponse": {
      "properties": {
        "user": {
          "$ref": "#/definitions/User"
        }
      }
    },
    "ErrorResponse": {
      "type": "object",
      "properties": {
        "message": {
          "description": "a human-readable explanation of the error",
          "type": "string",
          "example": "Bad Authentication data."
        },
        "status": {
          "description": "the HTTP response code",
          "type": "integer",
          "example": "401"
        },
        "type": {
          "description": "Type of the error message",
          "type": "string",
          "example": "OAuthException"
        }
      }
    },
    "SignupRequest": {
      "properties": {
        "email": {
          "description": "User's email",
          "type": "string",
          "example": "js@example.org"
        },
        "firstname": {
          "description": "User's first name",
          "type": "string",
          "example": "John"
        },
        "lastname": {
          "description": "User's last name",
          "type": "string",
          "example": "Smith"
        },
        "password": {
          "description": "User's password",
          "type": "string",
          "example": "some_password"
        }
      }
    },
    "SignupResponse": {
      "properties": {
        "user": {
          "$ref": "#/definitions/User"
        }
      }
    },
    "User": {
      "properties": {
        "firstname": {
          "description": "User's first name",
          "type": "string",
          "example": "John"
        },
        "key": {
          "description": "User's API Key that the client needs to provide when making API calls.",
          "type": "string",
          "example": "some_api_key"
        },
        "lastname": {
          "description": "User's last name",
          "type": "string",
          "example": "Smith"
        },
        "sid": {
          "description": "User's session ID",
          "type": "string",
          "example": "6c160004-35c8-45b1-a4da-0ab67076afde"
        },
        "uid": {
          "description": "User's ID in the system",
          "type": "integer",
          "example": "545648798"
        }
      }
    }
  },
  "securityDefinitions": {
    "APIKeyHeader": {
      "type": "apiKey",
      "name": "X-API-Key",
      "in": "header"
    },
    "APIKeyQueryParam": {
      "type": "apiKey",
      "name": "api_key",
      "in": "query"
    }
  }
}`))
	FlatSwaggerJSON = json.RawMessage([]byte(`{
  "schemes": [
    "http"
  ],
  "swagger": "2.0",
  "info": {
    "description": "Balda GameServer API methods and models\u003cbr\u003e \u003ch3\u003eHeaders\u003c/h3\u003e\u003cbr/\u003e \u003ctable\u003e \u003ctr\u003e\u003ctd\u003e\u003cb\u003e\u003ci\u003eX-API-Key\u003c/i\u003e\u003c/b\u003e\u003c/td\u003e\u003ctd\u003eAPI key is a special token that the client needs to provide when making API calls.\u003c/td\u003e\u003c/tr\u003e \u003ctr\u003e\u003ctd\u003e\u003cb\u003e\u003ci\u003eX-API-User\u003c/i\u003e\u003c/b\u003e\u003c/td\u003e\u003ctd\u003eUser's ID\u003c/td\u003e\u003c/tr\u003e \u003ctr\u003e\u003ctd\u003e\u003cb\u003e\u003ci\u003eX-API-Session\u003c/i\u003e\u003c/b\u003e\u003c/td\u003e\u003ctd\u003eSession ID\u003c/td\u003e\u003c/tr\u003e \u003c/table\u003e",
    "title": "Balda GameServer",
    "version": "1.0.0"
  },
  "host": "127.0.0.1:9666",
  "basePath": "/balda/api/v1",
  "paths": {
    "/auth": {
      "post": {
        "security": [
          {
            "APIKeyHeader": []
          }
        ],
        "consumes": [
          "application/json"
        ],
        "tags": [
          "Application"
        ],
        "summary": "Auth request",
        "parameters": [
          {
            "name": "body",
            "in": "body",
            "schema": {
              "$ref": "#/definitions/AuthRequest"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "Response for auth request",
            "schema": {
              "$ref": "#/definitions/AuthResponse"
            }
          },
          "401": {
            "description": "Error when auth",
            "schema": {
              "$ref": "#/definitions/ErrorResponse"
            }
          }
        }
      }
    },
    "/signup": {
      "post": {
        "consumes": [
          "application/json"
        ],
        "tags": [
          "Application"
        ],
        "summary": "Sign-up request",
        "parameters": [
          {
            "name": "body",
            "in": "body",
            "schema": {
              "$ref": "#/definitions/SignupRequest"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "Response for signup request",
            "schema": {
              "$ref": "#/definitions/SignupResponse"
            }
          },
          "400": {
            "description": "Error when signup",
            "schema": {
              "$ref": "#/definitions/ErrorResponse"
            }
          }
        }
      }
    }
  },
  "definitions": {
    "AuthRequest": {
      "properties": {
        "email": {
          "description": "User's email",
          "type": "string",
          "example": "js@example.org"
        },
        "firstname": {
          "description": "User's first name",
          "type": "string",
          "example": "John"
        },
        "lastname": {
          "description": "User's last name",
          "type": "string",
          "example": "Smith"
        },
        "password": {
          "description": "User's password",
          "type": "string",
          "example": "some_password"
        }
      }
    },
    "AuthResponse": {
      "properties": {
        "user": {
          "$ref": "#/definitions/User"
        }
      }
    },
    "ErrorResponse": {
      "type": "object",
      "properties": {
        "message": {
          "description": "a human-readable explanation of the error",
          "type": "string",
          "example": "Bad Authentication data."
        },
        "status": {
          "description": "the HTTP response code",
          "type": "integer",
          "example": "401"
        },
        "type": {
          "description": "Type of the error message",
          "type": "string",
          "example": "OAuthException"
        }
      }
    },
    "SignupRequest": {
      "properties": {
        "email": {
          "description": "User's email",
          "type": "string",
          "example": "js@example.org"
        },
        "firstname": {
          "description": "User's first name",
          "type": "string",
          "example": "John"
        },
        "lastname": {
          "description": "User's last name",
          "type": "string",
          "example": "Smith"
        },
        "password": {
          "description": "User's password",
          "type": "string",
          "example": "some_password"
        }
      }
    },
    "SignupResponse": {
      "properties": {
        "user": {
          "$ref": "#/definitions/User"
        }
      }
    },
    "User": {
      "properties": {
        "firstname": {
          "description": "User's first name",
          "type": "string",
          "example": "John"
        },
        "key": {
          "description": "User's API Key that the client needs to provide when making API calls.",
          "type": "string",
          "example": "some_api_key"
        },
        "lastname": {
          "description": "User's last name",
          "type": "string",
          "example": "Smith"
        },
        "sid": {
          "description": "User's session ID",
          "type": "string",
          "example": "6c160004-35c8-45b1-a4da-0ab67076afde"
        },
        "uid": {
          "description": "User's ID in the system",
          "type": "integer",
          "example": "545648798"
        }
      }
    }
  },
  "securityDefinitions": {
    "APIKeyHeader": {
      "type": "apiKey",
      "name": "X-API-Key",
      "in": "header"
    },
    "APIKeyQueryParam": {
      "type": "apiKey",
      "name": "api_key",
      "in": "query"
    }
  }
}`))
}
