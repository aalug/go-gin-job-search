package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	mockdb "github.com/aalug/go-gin-job-search/db/mock"
	db "github.com/aalug/go-gin-job-search/db/sqlc"
	"github.com/aalug/go-gin-job-search/utils"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

type eqCreateEmployerParamsMatcher struct {
	params   db.CreateEmployerParams
	password string
}

func (e eqCreateEmployerParamsMatcher) Matches(arg interface{}) bool {
	params, ok := arg.(db.CreateEmployerParams)
	if !ok {
		return false
	}

	err := utils.CheckPassword(e.password, params.HashedPassword)
	if err != nil {
		return false
	}

	e.params.HashedPassword = params.HashedPassword
	return reflect.DeepEqual(e.params, params)
}

func (e eqCreateEmployerParamsMatcher) String() string {
	return fmt.Sprintf("matches arg %v and password %v", e.params, e.password)
}

func EqCreateEmployerParams(arg db.CreateEmployerParams, password string) gomock.Matcher {
	return eqCreateEmployerParamsMatcher{arg, password}
}

func TestCreateEmployerAPI(t *testing.T) {
	employer, password, company := generateRandomEmployerAndCompany(t)

	requestBody := gin.H{
		"email":            employer.Email,
		"full_name":        employer.FullName,
		"password":         password,
		"company_name":     company.Name,
		"company_industry": company.Industry,
		"company_location": company.Location,
	}

	testCases := []struct {
		name          string
		body          gin.H
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: requestBody,
			buildStubs: func(store *mockdb.MockStore) {
				companyParams := db.CreateCompanyParams{
					Name:     company.Name,
					Industry: company.Industry,
					Location: company.Location,
				}
				store.EXPECT().
					CreateCompany(gomock.Any(), gomock.Eq(companyParams)).
					Times(1).
					Return(company, nil)
				store.EXPECT().
					CreateEmployer(gomock.Any(), gomock.Any()).
					Times(1).
					Return(employer, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)
				requireBodyMatchEmployerAndCompany(t, recorder.Body, employer, company)
			},
		},
		{
			name: "Internal Server Error CreateCompany",
			body: requestBody,
			buildStubs: func(store *mockdb.MockStore) {
				companyParams := db.CreateCompanyParams{
					Name:     company.Name,
					Industry: company.Industry,
					Location: company.Location,
				}
				store.EXPECT().
					CreateCompany(gomock.Any(), gomock.Eq(companyParams)).
					Times(1).
					Return(db.Company{}, sql.ErrConnDone)
				store.EXPECT().
					CreateEmployer(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name: "Internal Server Error CreateEmployer",
			body: requestBody,
			buildStubs: func(store *mockdb.MockStore) {
				companyParams := db.CreateCompanyParams{
					Name:     company.Name,
					Industry: company.Industry,
					Location: company.Location,
				}
				store.EXPECT().
					CreateCompany(gomock.Any(), gomock.Eq(companyParams)).
					Times(1).
					Return(company, nil)
				store.EXPECT().
					CreateEmployer(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Employer{}, sql.ErrConnDone)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name: "Invalid Email",
			body: gin.H{
				"email":            "invalid",
				"full_name":        employer.FullName,
				"password":         password,
				"company_name":     company.Name,
				"company_industry": company.Industry,
				"company_location": company.Location,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					CreateCompany(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					CreateEmployer(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "Duplicated Company Name",
			body: requestBody,
			buildStubs: func(store *mockdb.MockStore) {
				params := db.CreateCompanyParams{
					Name:     company.Name,
					Industry: company.Industry,
					Location: company.Location,
				}
				store.EXPECT().
					CreateCompany(gomock.Any(), gomock.Eq(params)).
					Times(1).
					Return(db.Company{}, &pq.Error{Code: "23505"})
				store.EXPECT().
					CreateEmployer(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name: "Duplicated Email",
			body: requestBody,
			buildStubs: func(store *mockdb.MockStore) {
				params := db.CreateCompanyParams{
					Name:     company.Name,
					Industry: company.Industry,
					Location: company.Location,
				}
				store.EXPECT().
					CreateCompany(gomock.Any(), gomock.Eq(params)).
					Times(1).
					Return(company, nil)
				store.EXPECT().
					CreateEmployer(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Employer{}, &pq.Error{Code: "23505"})
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
	}
	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/employers"
			req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			server.router.ServeHTTP(recorder, req)

			tc.checkResponse(recorder)
		})
	}
}

func TestLoginEmployerAPI(t *testing.T) {
	employer, password, company := generateRandomEmployerAndCompany(t)

	testCases := []struct {
		name          string
		body          gin.H
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: gin.H{
				"email":    employer.Email,
				"password": password,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(employer, nil)
				store.EXPECT().
					GetCompanyByID(gomock.Any(), gomock.Eq(employer.CompanyID)).
					Times(1).
					Return(company, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "Employer Not Found",
			body: gin.H{
				"email":    employer.Email,
				"password": password,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(db.Employer{}, sql.ErrNoRows)
				store.EXPECT().
					GetCompanyByID(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "Internal Server Error GetEmployerByEmail",
			body: gin.H{
				"email":    employer.Email,
				"password": password,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Employer{}, sql.ErrConnDone)
				store.EXPECT().
					GetCompanyByID(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name: "Internal Server Error GetCompanyByID",
			body: gin.H{
				"email":    employer.Email,
				"password": password,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(employer, nil)
				store.EXPECT().
					GetCompanyByID(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Company{}, sql.ErrConnDone)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name: "Company Not Found",
			body: gin.H{
				"email":    employer.Email,
				"password": password,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(employer, nil)
				store.EXPECT().
					GetCompanyByID(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Company{}, sql.ErrNoRows)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "Invalid Email",
			body: gin.H{
				"email":    "invalid",
				"password": password,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					GetCompanyByID(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "Incorrect Password",
			body: gin.H{
				"email":    employer.Email,
				"password": fmt.Sprintf("%d, %s", utils.RandomInt(1, 1000), utils.RandomString(10)),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(employer, nil)
				store.EXPECT().
					GetCompanyByID(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "Password Too Short",
			body: gin.H{
				"email":    employer.Email,
				"password": "abc",
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					GetCompanyByID(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
	}
	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/employers/login"
			req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			server.router.ServeHTTP(recorder, req)

			tc.checkResponse(recorder)
		})
	}
}

// generateRandomEmployer create a random employer and company
func generateRandomEmployerAndCompany(t *testing.T) (db.Employer, string, db.Company) {
	password := utils.RandomString(6)
	hashedPassword, err := utils.HashPassword(password)
	require.NoError(t, err)

	company := db.Company{
		ID:       utils.RandomInt(1, 100),
		Name:     utils.RandomString(5),
		Industry: utils.RandomString(5),
		Location: utils.RandomString(6),
	}

	employer := db.Employer{
		ID:             utils.RandomInt(1, 100),
		CompanyID:      company.ID,
		FullName:       utils.RandomString(5),
		Email:          utils.RandomEmail(),
		HashedPassword: hashedPassword,
		CreatedAt:      time.Now(),
	}

	return employer, password, company
}

// requireBodyMatchEmployerAndCompany checks if the body of the response matches the employer and company
func requireBodyMatchEmployerAndCompany(t *testing.T, body *bytes.Buffer, employer db.Employer, company db.Company) {
	data, err := io.ReadAll(body)
	require.NoError(t, err)

	var response employerResponse
	err = json.Unmarshal(data, &response)

	require.NoError(t, err)
	require.NotZero(t, response.EmployerID)
	require.Equal(t, employer.Email, response.Email)
	require.Equal(t, employer.FullName, response.FullName)
	require.Equal(t, employer.CompanyID, response.CompanyID)
	require.Equal(t, company.Name, response.CompanyName)
	require.Equal(t, company.Industry, response.CompanyIndustry)
	require.Equal(t, company.Location, response.CompanyLocation)
	require.WithinDuration(t, employer.CreatedAt, response.EmployerCreatedAt, time.Second)
}
