package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	. "tophone.evanjon.es/internal/common"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"github.com/ttacon/libphonenumber"
)

type app struct {
	data dataLayer
	sms  smsLayer
}

const (
	PER_PAGE               = 20
	KEY_USER_SESSION_TOKEN = iota
)

type dataLayer interface {
	UserGet(token string) (User, error)
	UserGetByNumber(number string) (User, error)
	UserLogin(email, pass string) (User, error)
	UserCreate(email, pass, phone string) (User, error)
	NoteGetList(user User, page, count int) ([]Note, bool, error)
	NoteGetLatest(user User) (Note, error)
	NoteGetLatestWithTime(user User, t time.Duration) (Note, error)
	NoteCreate(user User, text string) (Note, error)
}

type smsLayer interface {
	Send(number, text string) error
}

func AppDefault(data dataLayer, sms smsLayer) app {
	return app{
		data,
		sms,
	}
}

func (app app) HookSMS(c *gin.Context) {
	var payload struct {
		From, To, Text string
	}

	err := c.Bind(&payload)
	if err != nil {
		app.error(c, err)
		return
	}

	user, err := app.data.UserGetByNumber(payload.From)
	if err != nil {
		app.error(c, err)
		return
	}

	_, err = app.data.NoteCreate(user, payload.Text)
	if err != nil {
		app.error(c, err)
		return
	}

	c.String(http.StatusOK, "message received")
}

func (app app) NoteCreate(c *gin.Context) {
	var payload struct {
		Text string
	}

	err := c.Bind(&payload)
	if err != nil {
		app.error(c, err)
		return
	}

	user, err := app.currentUser(c)
	if err != nil {
		app.error(c, errors.New("not logged in; or something else terribly wrong"))
		return
	}

	note, err := app.data.NoteCreate(user, payload.Text)
	if err != nil {
		app.error(c, err)
		return
	}

	if err := app.sms.Send(user.Phone(), note.Text()); err != nil {
		app.error(c, err)
		return
	}

	c.Redirect(http.StatusTemporaryRedirect, "/")
}

func (app app) NoteCreateCLI(c *gin.Context) {
	var payload struct {
		Token, Text string
	}

	err := c.Bind(&payload)
	if err != nil {
		app.error(c, err)
		return
	}

	user, err := app.currentUserFromToken(payload.Token)
	if err != nil {
		app.error(c, errors.New("not logged in; or something else terribly wrong"))
		return
	}

	note, err := app.data.NoteCreate(user, payload.Text)
	if err != nil {
		app.error(c, err)
		return
	}

	if err := app.sms.Send(user.Phone(), note.Text()); err != nil {
		app.error(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"Message": "complete"})
}

func (app app) NoteLatestCLI(c *gin.Context) {
	var payload struct {
		Token string
	}

	err := c.Bind(&payload)
	if err != nil {
		app.error(c, err)
		return
	}

	user, err := app.currentUserFromToken(payload.Token)
	if err != nil {
		app.error(c, errors.New("not logged in; or something else terribly wrong"))
		return
	}

	note, err := app.data.NoteGetLatest(user)
	if err != nil {
		app.error(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"Message": "complete", "Note": note})
}

func (app app) UserLogin(c *gin.Context) {
	var payload struct {
		Email, Password string
	}

	err := c.Bind(&payload)
	if err != nil {
		app.error(c, err)
		return
	}

	user, err := app.data.UserLogin(payload.Email, payload.Password)
	if err != nil {
		app.error(c, err)
		return
	}

	s := sessions.Default(c)
	s.Set(KEY_USER_SESSION_TOKEN, user.Token())
	if err := s.Save(); err != nil {
		app.error(c, err)
		return
	}

	c.Redirect(http.StatusTemporaryRedirect, "/")
}

func (app app) UserLoginCLI(c *gin.Context) {
	var payload struct {
		Email, Password string
	}

	err := c.Bind(&payload)
	if err != nil {
		app.error(c, err)
		return
	}

	user, err := app.data.UserLogin(payload.Email, payload.Password)
	if err != nil {
		app.error(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"Token": user.Token()})
}

func (app app) UserCreate(c *gin.Context) {
	var payload struct {
		Email, Password, Verify, Phone string
	}

	err := c.Bind(&payload)
	if err != nil {
		app.error(c, err)
		return
	}

	if payload.Password != payload.Verify || payload.Password == "" {
		app.error(c, errors.New("invalid password; either not equal or no password entered"))
		return
	}

	phone, err := libphonenumber.Parse(payload.Phone, "US")
	if err != nil {
		app.error(c, err)
		return
	} else if !libphonenumber.IsValidNumber(phone) {
		app.error(c, errors.New("invalid phone number; try again"))
		return
	}
	full := fmt.Sprintf("%d%d", phone.GetCountryCode(), phone.GetNationalNumber())

	user, err := app.data.UserCreate(payload.Email, payload.Password, full)
	if err != nil {
		app.error(c, err)
		return
	}

	s := sessions.Default(c)
	s.Set(KEY_USER_SESSION_TOKEN, user.Token())
	if err := s.Save(); err != nil {
		app.error(c, err)
		return
	}

	c.Redirect(http.StatusTemporaryRedirect, "/")
}

func (app app) UserCreateCLI(c *gin.Context) {
	var payload struct {
		Email, Password, Verify, Phone string
	}

	err := c.Bind(&payload)
	if err != nil {
		app.error(c, err)
		return
	}

	if payload.Password != payload.Verify || payload.Password == "" {
		app.error(c, errors.New("invalid password; either not equal or no password entered"))
		return
	}

	phone, err := libphonenumber.Parse(payload.Phone, "US")
	if err != nil {
		app.error(c, err)
		return
	} else if !libphonenumber.IsValidNumber(phone) {
		app.error(c, errors.New("invalid phone number; try again"))
		return
	}
	full := fmt.Sprintf("%d%d", phone.GetCountryCode(), phone.GetNationalNumber())

	user, err := app.data.UserCreate(payload.Email, payload.Password, full)
	if err != nil {
		app.error(c, err)
		return
	}

	s := sessions.Default(c)
	s.Set(KEY_USER_SESSION_TOKEN, user.Token())
	if err := s.Save(); err != nil {
		app.error(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"Token": user.Token()})
}

func (app app) UserUpdate(c *gin.Context) {
	var payload struct {
		Email, Password, Verify, Phone string
	}

	err := c.Bind(&payload)
	if err != nil {
		app.error(c, err)
		return
	}

	if payload.Password != payload.Verify {
		app.error(c, errors.New("invalid password; not equal"))
		return
	}

	user, err := app.currentUser(c)
	if err != nil {
		app.error(c, err)
		return
	}

	if payload.Email != "" {
		user.SetEmail(payload.Email)
	}

	if payload.Password != "" {
		user.SetPass(payload.Password)
	}

	if payload.Phone != "" {
		phone, err := libphonenumber.Parse(payload.Phone, "US")
		if err != nil {
			app.error(c, err)
			return
		} else if !libphonenumber.IsValidNumber(phone) {
			app.error(c, errors.New("invalid phone number; try again"))
			return
		}
		user.SetPhone(fmt.Sprintf("%d%d", phone.GetCountryCode(), phone.GetNationalNumber()))
	}

	err = user.Save()
	if err != nil {
		app.error(c, err)
		return
	}

	s := sessions.Default(c)
	s.Set(KEY_USER_SESSION_TOKEN, user.Token())
	if err := s.Save(); err != nil {
		app.error(c, err)
		return
	}

	c.Redirect(http.StatusTemporaryRedirect, "/")
}

func (app app) UserLogout(c *gin.Context) {
	s := sessions.Default(c)
	s.Set(KEY_USER_SESSION_TOKEN, nil)
	if err := s.Save(); err != nil {
		app.error(c, err)
		return
	}

	c.Redirect(http.StatusTemporaryRedirect, "/")
}

func (app app) Page(c *gin.Context) {
	user, err := app.currentUser(c)
	if err != nil {
		// Not really an error, just we don't currently have a user stored in
		// session.
		c.HTML(http.StatusOK, "main.html", gin.H{
			"HasUser": false,
		})
		return
	}

	page := 0
	notes, hasMore, err := app.data.NoteGetList(user, page, PER_PAGE)
	if err != nil {
		app.error(c, err)
		return
	}

	latest, err := app.data.NoteGetLatestWithTime(user, 5*time.Minute)
	if err != nil {
		app.error(c, err)
		return
	}

	c.HTML(http.StatusOK, "main.html", gin.H{
		"HasUser":      true,
		"User":         user,
		"Notes":        notes,
		"NotesHasMore": hasMore,
		"Latest":       latest,
	})
}

func (app app) NoteListJSON(c *gin.Context) {
	user, err := app.currentUser(c)
	if err != nil {
		// Not really an error, just we don't currently have a user stored in
		// session.
		app.error(c, errors.New("no user"))
		return
	}

	strPage := c.Param("page")
	page, err := strconv.Atoi(strPage)
	if err != nil {
		app.error(c, err)
		return
	}

	notes, hasMore, err := app.data.NoteGetList(user, page, PER_PAGE)
	if err != nil {
		app.error(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"HasUser":      true,
		"User":         user,
		"Notes":        notes,
		"NotesHasMore": hasMore,
	})
}

func (app app) currentUser(c *gin.Context) (User, error) {
	s := sessions.Default(c)
	token, ok := s.Get(KEY_USER_SESSION_TOKEN).(string)
	if !ok {
		return nil, errors.New("session key type assertion failed")
	}
	return app.currentUserFromToken(token)
}

func (app app) currentUserFromToken(token string) (User, error) {
	user, err := app.data.UserGet(token)
	return user, err
}

func (app app) error(c *gin.Context, err error) {
	c.String(http.StatusInternalServerError, err.Error())
}
