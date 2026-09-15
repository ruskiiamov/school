package admin

import (
	"context"
	"net/http"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/server/web"
)

type Handler struct {
	base    *web.Base
	auth    *auth.Service
	school  *school.Service
	created *credentialsStore
}

func New(base *web.Base, authService *auth.Service, schoolService *school.Service) *Handler {
	return &Handler{
		base:    base,
		auth:    authService,
		school:  schoolService,
		created: newCredentialsStore(),
	}
}

func (h *Handler) Routes(mux *http.ServeMux) {
	mux.Handle("GET "+classesPath, h.admin(h.classesList))
	mux.Handle("POST "+classesPath, h.admin(h.classCreate))
	mux.Handle("GET "+classesPath+"/{id}", h.admin(h.classShow))
	mux.Handle("POST "+classesPath+"/{id}", h.admin(h.classUpdate))
	mux.Handle("POST "+classesPath+"/{id}/deactivate", h.admin(h.classDeactivate))
	mux.Handle("POST "+classesPath+"/{id}/activate", h.admin(h.classActivate))
	mux.Handle("POST "+classesPath+"/{id}/students", h.admin(h.classStudentAdd))
	mux.Handle("POST "+classesPath+"/{id}/students/{sid}/remove", h.admin(h.classStudentRemove))
	mux.Handle("POST "+classesPath+"/{id}/assignments", h.admin(h.classAssign))
	mux.Handle("POST "+classesPath+"/{id}/assignments/{aid}/remove", h.admin(h.classAssignmentRemove))

	for _, section := range userSections {
		mux.Handle("GET "+section.path, h.admin(h.usersList(section)))
		mux.Handle("POST "+section.path, h.admin(h.userCreate(section)))
		mux.Handle("GET "+section.path+"/{id}/created", h.admin(h.userCreated(section)))
		mux.Handle("POST "+section.path+"/{id}", h.admin(h.userUpdate(section)))
		mux.Handle("POST "+section.path+"/{id}/deactivate", h.admin(h.userSetActive(section, false)))
		mux.Handle("POST "+section.path+"/{id}/activate", h.admin(h.userSetActive(section, true)))
	}

	mux.Handle("POST "+parentsSection.path+"/{id}/children", h.admin(h.parentChildAdd))
	mux.Handle("POST "+parentsSection.path+"/{id}/children/{sid}/remove", h.admin(h.parentChildRemove))

	mux.Handle("GET "+passwordResetPath, h.admin(h.passwordResetList))
	mux.Handle("POST "+passwordResetPath+"/{id}", h.admin(h.passwordReset))
	mux.Handle("GET "+passwordResetPath+"/{id}/created", h.admin(h.passwordResetCreated))

	mux.Handle("GET "+substitutionsPath, h.admin(h.substitutionsList))
	mux.Handle("POST "+substitutionsPath, h.admin(h.substitutionCreate))
	mux.Handle("POST "+substitutionsPath+"/{id}", h.admin(h.substitutionUpdate))
	mux.Handle("POST "+substitutionsPath+"/{id}/delete", h.admin(h.substitutionDelete))

	mux.Handle("GET "+subjectsPath, h.admin(h.subjectsList))
	mux.Handle("POST "+subjectsPath, h.admin(h.subjectCreate))
	mux.Handle("POST "+subjectsPath+"/{id}", h.admin(h.subjectUpdate))
	mux.Handle("POST "+subjectsPath+"/{id}/deactivate", h.admin(h.subjectDeactivate))
	mux.Handle("POST "+subjectsPath+"/{id}/activate", h.admin(h.subjectActivate))

	mux.Handle("GET "+workTypesPath, h.admin(h.workTypesList))
	mux.Handle("POST "+workTypesPath, h.admin(h.workTypeCreate))
	mux.Handle("POST "+workTypesPath+"/{id}", h.admin(h.workTypeUpdate))
	mux.Handle("POST "+workTypesPath+"/{id}/deactivate", h.admin(h.workTypeDeactivate))
	mux.Handle("POST "+workTypesPath+"/{id}/activate", h.admin(h.workTypeActivate))
	mux.Handle("POST "+workTypesPath+"/{id}/up", h.admin(h.workTypeUp))
	mux.Handle("POST "+workTypesPath+"/{id}/down", h.admin(h.workTypeDown))
}

func (h *Handler) admin(fn http.HandlerFunc) http.Handler {
	return h.base.RequireAuth(web.RequireRole(auth.RoleAdmin)(fn))
}

func (h *Handler) activeUser(ctx context.Context, id int64, role auth.Role) (auth.User, error) {
	user, err := h.auth.UserByID(ctx, id)
	if err != nil {
		return auth.User{}, err
	}

	if user.Role != role || !user.Active {
		return auth.User{}, auth.ErrNotFound
	}

	return user, nil
}
