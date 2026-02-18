package handler

import (
	"xcore-example/pkg/xcore"

	"github.com/gorilla/mux"
)

type AuthHandler struct {
}

func (h *AuthHandler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/login", xcore.RouteWrapper(h.Login)).Methods("POST").Name("auth.login")
	r.HandleFunc("/register", xcore.RouteWrapper(h.Register)).Methods("POST").Name("auth.register")
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

func (h *AuthHandler) Login(rb *xcore.ResponseBuilder) {
	// Implement login logic here
}

type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Username string `json:"username" validate:"required,min=3,max=100"`
	Password string `json:"password" validate:"required,min=6"`
	Name     string `json:"name" validate:"omitempty,max=100"`
}

func (h *AuthHandler) Register(rb *xcore.ResponseBuilder) {
	var req RegisterRequest
	if err := rb.ParseJSON(&req); err != nil {
		rb.HandleError(err)
		return
	}

	rb.OK(req)
}
