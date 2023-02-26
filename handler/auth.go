package handler

import (
	"net/http"
	"strings"

	"github.com/juliotorresmoreno/SpecialistTalk/configs"
	"github.com/juliotorresmoreno/SpecialistTalk/db"
	"github.com/juliotorresmoreno/SpecialistTalk/helper"
	"github.com/juliotorresmoreno/SpecialistTalk/model"
	"github.com/juliotorresmoreno/SpecialistTalk/services"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
}

type POSTSingUpPayload struct {
	Email    string `yaml:"email"   `
	Password string `yaml:"password"`
	Username string `yaml:"username"`
	Name     string `yaml:"name"    `
	LastName string `yaml:"lastname"`
}

func (el *AuthHandler) POSTSingUp(c echo.Context) error {
	conn, err := db.GetConnectionPool()
	if err != nil {
		return helper.MakeHTTPError(http.StatusInternalServerError, err)
	}

	p := &POSTSingUpPayload{}
	err = c.Bind(p)
	if err != nil {
		return helper.MakeHTTPError(http.StatusBadRequest, "body has not valid format")
	}

	u := &model.User{}
	u.Email = p.Email
	exists, err := conn.Get(u)
	if err != nil {
		return helper.MakeHTTPError(http.StatusInternalServerError, err)
	}
	if exists {
		return helper.MakeHTTPError(http.StatusUnauthorized, "user already exists")
	}

	u.Name = p.Name
	u.LastName = p.LastName
	u.Username = p.Username
	u.ValidPassword = p.Password
	u.Owner = p.Username
	if err != nil {
		return helper.MakeHTTPError(http.StatusBadRequest, err)
	}

	if err = u.Check(); err != nil {
		if strings.Contains(err.Error(), "ValidPassword") {
			return helper.MakeHTTPError(
				http.StatusBadRequest,
				"password: the policy is not followed",
			)
		}
		return helper.MakeHTTPError(
			http.StatusBadRequest,
			strings.ToLower(err.Error()),
		)
	}

	err = u.SetPassword(p.Password)
	if err != nil {
		return helper.MakeHTTPError(http.StatusInternalServerError, err)
	}

	_, err = conn.Table(u.TableName()).InsertOne(u)

	if err != nil {
		return helper.MakeHTTPError(http.StatusInternalServerError, err)
	}

	return helper.MakeSession(c, u)
}

type POSTLoginPayload struct {
	Email    string `yaml:"email"`
	Password string `yaml:"password"`
}

func (el *AuthHandler) POSTLogin(c echo.Context) error {
	conn, err := db.GetConnectionPool()
	if err != nil {
		return helper.MakeHTTPError(http.StatusInternalServerError, err)
	}

	p := &POSTLoginPayload{}
	err = c.Bind(p)
	if err != nil {
		return helper.MakeHTTPError(http.StatusBadRequest, err)
	}
	u := &model.User{}
	u.Email = p.Email
	_, err = conn.Get(u)
	if err != nil {
		return helper.MakeHTTPError(http.StatusInternalServerError, err)
	}
	err = bcrypt.CompareHashAndPassword(
		[]byte(u.Password),
		[]byte(p.Password),
	)
	if err != nil {
		return helper.MakeHTTPError(http.StatusUnauthorized, "password: password is not valid")
	}

	return helper.MakeSession(c, u)
}

type CredentialsRecovery struct {
	Email string
}

func (el *AuthHandler) POSTRecovery(c echo.Context) error {
	conn, err := db.GetConnectionPool()
	if err != nil {
		return helper.MakeHTTPError(http.StatusInternalServerError, err)
	}

	p := &CredentialsRecovery{}
	err = c.Bind(p)
	if err != nil {
		return helper.MakeHTTPError(http.StatusInternalServerError, err)
	}

	token := helper.StringWithAlphanumCharset(40)
	if p.Email == "" {
		return helper.MakeHTTPError(http.StatusNotAcceptable, "email is required")
	}
	u := &model.User{RecoveryToken: token}
	q := &model.User{Email: p.Email}

	_, err = conn.Omit("owner").Update(u, q)
	if err != nil {
		return helper.MakeHTTPError(http.StatusInternalServerError, err)
	}
	return c.String(http.StatusNoContent, "")
}

type POSTResetPayload struct {
	Password string
	Token    string
}

func (el *AuthHandler) POSTReset(c echo.Context) error {
	conn, err := db.GetConnectionPool()
	if err != nil {
		return helper.MakeHTTPError(http.StatusInternalServerError, err)
	}

	p := &POSTResetPayload{}
	err = c.Bind(p)
	if err != nil {
		return helper.MakeHTTPError(http.StatusInternalServerError, err)
	}
	if p.Token == "-" || p.Token == "" {
		return helper.MakeHTTPError(http.StatusNotAcceptable, "token is required")
	}
	q := &model.User{RecoveryToken: p.Token}
	u := &model.User{}
	_, err = conn.Get(u)
	if err != nil {
		return helper.MakeHTTPError(http.StatusNotAcceptable, err)
	}

	err = u.SetPassword(p.Password)
	if err != nil {
		return helper.MakeHTTPError(http.StatusNotAcceptable, err)
	}
	u.RecoveryToken = "-"
	if err := u.Check(); err != nil {
		return helper.MakeHTTPError(http.StatusNotAcceptable, err)
	}
	_, err = conn.Omit("owner").Update(u, q)
	if err != nil {
		return helper.MakeHTTPError(http.StatusInternalServerError, err)
	}
	return c.String(204, "")
}

func (that *AuthHandler) GETSession(c echo.Context) error {
	session, err := helper.ValidateSession(c)
	if err != nil {
		return helper.HTTPErrorUnauthorized
	}

	conf := configs.GetConfig()
	conn, err := db.GetConnectionPoolWithSession(conf.Database, session)
	if err != nil {
		return err
	}

	u := &model.User{}
	_, err = conn.Where("id = ?", session.ID).Get(u)
	if err != nil {
		return helper.MakeHTTPError(http.StatusInternalServerError, err)
	}
	return c.JSON(200, u)
}

func (that *AuthHandler) DELETESession(c echo.Context) error {
	session, err := helper.ValidateSession(c)
	if err != nil {
		return helper.HTTPStatusNotContent
	}

	token := helper.GetToken(c)
	redisCli := services.GetPoolRedis()
	_ = redisCli.Del(token)

	dispatchDisconnect(session.Username)

	return helper.HTTPStatusNotContent
}

func AttachAuth(g *echo.Group) {
	c := AuthHandler{}
	g.DELETE("/session", c.DELETESession)
	g.GET("/session", c.GETSession)
	g.POST("/sing-up", c.POSTSingUp)
	g.POST("/sing-in", c.POSTLogin)
	g.POST("/recovery", c.POSTRecovery)
	g.POST("/reset", c.POSTReset)
}
