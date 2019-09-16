/*
 * Copyright 2019 Azz. All rights reserved.
 * Use of this source code is governed by a GPL-3.0
 * license that can be found in the LICENSE file.
 */

package kinoko_web

import (
	"encoding/json"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
)

type RequestCtx struct {
	// url query string
	QueryString map[string][]string

	// path variable parsed automatically by framework
	PathVariable map[string]string

	//original http request
	Request *http.Request

	// used for multipart form request, call Ctx.Request.ParseMultipartForm before using it
	// please resolve manually for big file
	MultipartForm *multipart.Form

	// directly access response writer
	ResponseWriter http.ResponseWriter

	// sql session is created for each request if necessary, using BeginTx to start a transaction
	SQL *SQLSession

	//customized properties by resolving request with RequestResolver
	Properties map[interface{}]interface{}
}

func NewRequestCtx(queryString map[string][]string, pathVariable map[string]string, request *http.Request, form *multipart.Form, responseWriter http.ResponseWriter) *RequestCtx {
	var session *SQLSession
	if sqlPropertiesHolder.SQL.Valid {
		session = newSQLSession(0, sqlPropertiesHolder.SQL.DefaultDataSource)
	}
	return &RequestCtx{
		QueryString:    queryString,
		PathVariable:   pathVariable,
		Request:        request,
		MultipartForm:  form,
		ResponseWriter: responseWriter,
		SQL:            session,
		Properties:     map[interface{}]interface{}{},
	}
}

func (c *RequestCtx) ParseBody(dst interface{}) error {
	ct := c.Request.Header.Get("Content-Type")
	contentType := strings.Split(ct, ";")[0]
	if strings.EqualFold(contentType, "application/json") {
		bytes, e := ioutil.ReadAll(c.Request.Body) //Note that ReadAll is not safe for oom attack
		defer func() {
			_ = c.Request.Body.Close()
		}()

		if e != nil {
			return e
		}

		e = json.Unmarshal(bytes, dst)
		if e != nil {
			return e
		}
		return nil
	}

	if strings.EqualFold(contentType, "application/x-www-form-urlencoded") {
		bytes, e := ioutil.ReadAll(c.Request.Body)
		defer func() {
			_ = c.Request.Body.Close()
		}()

		if e != nil {
			return e
		}

		values, e := url.ParseQuery(string(bytes))

		if e != nil {
			return e
		}

		//can be optimized by implement a decoder
		bytes, e = json.Marshal(values)

		if e != nil {
			return e
		}

		e = json.Unmarshal(bytes, dst)
		if e != nil {
			return e
		}

		return nil

	}

	logger.Info("content-type: '%s' is not supported yet\n", ct)
	return nil
}
