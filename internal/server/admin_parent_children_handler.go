package server

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/ruskiiamov/school/internal/auth"
	"github.com/ruskiiamov/school/internal/school"
	"github.com/ruskiiamov/school/internal/validation"
	"github.com/ruskiiamov/school/internal/view"
)

func (s *Server) parentChildAdd(w http.ResponseWriter, r *http.Request) {
	parent, ok := s.sectionUser(w, r, parentsSection)
	if !ok {
		return
	}

	err := s.addChild(r, parent.ID, formInt64(r, "student_id"))
	if errs, ok := formErrors(err); ok {
		s.renderChildrenError(w, r, parent, errs)
		return
	}
	if err != nil {
		s.handleServiceError(w, r, "add child", err)
		return
	}

	s.childrenDone(w, r, parent.ID)
}

func (s *Server) addChild(r *http.Request, parentID, studentID int64) error {
	_, err := s.activeUser(r.Context(), studentID, auth.RoleStudent)
	if errors.Is(err, auth.ErrNotFound) {
		return validation.Errors{"child": msgStudentUnknown}
	}
	if err != nil {
		return err
	}

	return s.school.AddChild(r.Context(), parentID, studentID)
}

func (s *Server) parentChildRemove(w http.ResponseWriter, r *http.Request) {
	parent, ok := s.sectionUser(w, r, parentsSection)
	if !ok {
		return
	}

	studentID, ok := pathValue(r, "sid")
	if !ok {
		http.NotFound(w, r)
		return
	}

	if err := s.school.RemoveChild(r.Context(), parent.ID, studentID); err != nil {
		s.handleServiceError(w, r, "remove child", err)
		return
	}

	s.childrenDone(w, r, parent.ID)
}

func (s *Server) childrenDone(w http.ResponseWriter, r *http.Request, parentID int64) {
	if isHTMX(r) {
		s.renderUsers(w, r, parentsSection, userForm{}, nil, userEdit{})
		return
	}

	s.redirect(w, r, parentsSection.path+encodeQuery(childrenValues(r, parentID)))
}

func (s *Server) renderChildrenError(w http.ResponseWriter, r *http.Request, parent auth.User, errs validation.Errors) {
	edit := userEdit{
		id:      parent.ID,
		form:    userForm{fullName: parent.FullName, login: parent.Login},
		entered: true,
		errs:    errs,
	}

	s.renderUsers(w, r, parentsSection, userForm{}, nil, edit)
}

func childQuery(r *http.Request) string {
	return strings.TrimSpace(r.URL.Query().Get("child"))
}

func childrenValues(r *http.Request, parentID int64) url.Values {
	values := listValues(strings.TrimSpace(r.URL.Query().Get("q")), queryInt64(r, "class"), showInactive(r))
	values.Set("edit", strconv.FormatInt(parentID, 10))

	if child := childQuery(r); child != "" {
		values.Set("child", child)
	}

	return values
}

type childrenData struct {
	byParent map[int64][]int64
	students map[int64]auth.User
	classes  map[int64]school.Class
}

func (s *Server) loadChildren(r *http.Request) (childrenData, error) {
	ctx := r.Context()

	byParent, err := s.school.Children(ctx)
	if err != nil {
		return childrenData{}, err
	}

	students, err := s.auth.Users(ctx, auth.UserFilter{Role: auth.RoleStudent, IncludeInactive: true})
	if err != nil {
		return childrenData{}, err
	}

	classes, err := s.school.StudentClasses(ctx, s.school.CurrentYear())
	if err != nil {
		return childrenData{}, err
	}

	data := childrenData{byParent: byParent, students: make(map[int64]auth.User, len(students)), classes: classes}
	for _, student := range students {
		data.students[student.ID] = student
	}

	return data, nil
}

func (d childrenData) names(parentID int64) string {
	var names []string

	for _, student := range d.children(parentID) {
		names = append(names, student.FullName)
	}

	return strings.Join(names, ", ")
}

func (d childrenData) children(parentID int64) []auth.User {
	var children []auth.User

	for _, id := range d.byParent[parentID] {
		if student, ok := d.students[id]; ok {
			children = append(children, student)
		}
	}

	return children
}

func (d childrenData) member(student auth.User, removeAction string) view.MemberRow {
	return view.MemberRow{
		ID:           student.ID,
		FullName:     student.FullName,
		ClassName:    d.classes[student.ID].Name,
		Active:       student.Active,
		RemoveAction: removeAction,
	}
}

func (s *Server) childrenBlock(r *http.Request, data childrenData, parentID int64, message string) (*view.ChildrenBlock, error) {
	values := childrenValues(r, parentID)
	query := encodeQuery(values)
	child := childQuery(r)

	hidden := make(map[string]string, len(values))
	for name := range values {
		if name != "child" {
			hidden[name] = values.Get(name)
		}
	}

	block := &view.ChildrenBlock{
		Query:        child,
		Searched:     child != "",
		SearchAction: parentsSection.path,
		Hidden:       hidden,
		AddAction:    parentsSection.userPath(parentID, "/children") + query,
		Error:        message,
	}

	own := make(map[int64]struct{})

	for _, student := range data.children(parentID) {
		own[student.ID] = struct{}{}
		remove := parentsSection.userPath(parentID, "/children/"+strconv.FormatInt(student.ID, 10)+"/remove") + query
		block.Children = append(block.Children, data.member(student, remove))
	}

	if child == "" {
		return block, nil
	}

	found, err := s.auth.Users(r.Context(), auth.UserFilter{Role: auth.RoleStudent, Query: child})
	if err != nil {
		return nil, err
	}

	for _, student := range found {
		if _, taken := own[student.ID]; !taken {
			block.Results = append(block.Results, data.member(student, ""))
		}
	}

	return block, nil
}
