package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type TestCase struct {
	name          string
	req           SearchRequest
	expected      []User
	expectedCount int  // ожидание определённого количества пользователей
	checkCount    bool // флаг для проверки только количества
	isError       bool // флаг для проверки наличия ошибки
	checkUsers    bool // флаг для проверки возвращаемых Users
}

func SearchServerDummy(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("query")
	switch key {
	case "__401":
		w.WriteHeader(http.StatusUnauthorized)
	case "__500":
		w.WriteHeader(http.StatusInternalServerError)
	case "__badjson":
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"Error":`))
	case "__badorder":
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"Error":"ErrorBadOrderField"}`))
	case "__badjson_200":
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"Error":`))
	default:
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"Error":"unknown error"}`))
	}
}

func TestErrors(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(SearchServerDummy))
	defer ts.Close()

	testCases := []struct {
		name      string
		expectErr string
		searchReq SearchRequest
	}{
		{
			name:      "StatusUnauthorized",
			expectErr: "Ожидаем ошибку авторизации",
			searchReq: SearchRequest{
				Limit:      1,
				Offset:     0,
				Query:      "__401",
				OrderField: "Name",
				OrderBy:    OrderByAsc,
			},
		},
		{
			name:      "StatusInternalServerError",
			expectErr: "Ожидаем серверную ошибку",
			searchReq: SearchRequest{
				Limit:      1,
				Offset:     0,
				Query:      "__500",
				OrderField: "Name",
				OrderBy:    OrderByAsc,
			},
		},
		{
			name:      "InvalidJSON",
			expectErr: "Ожидаем ошибку при распаковке JSON",
			searchReq: SearchRequest{
				Limit:      1,
				Offset:     0,
				Query:      "__badjson",
				OrderField: "Name",
				OrderBy:    OrderByAsc,
			},
		},
		{
			name:      "ErrorBadOrderField",
			expectErr: "Ожидаем ошибку ErrorBadOrderField",
			searchReq: SearchRequest{
				Limit:      1,
				Offset:     0,
				Query:      "__badorder",
				OrderField: "Name",
				OrderBy:    OrderByAsc,
			},
		},
		{
			name:      "unknownError",
			expectErr: "Ожидаем ошибку Unknown Error",
			searchReq: SearchRequest{
				Limit:      1,
				Offset:     0,
				Query:      "__",
				OrderField: "Name",
				OrderBy:    OrderByAsc,
			},
		},
		{
			name:      "unknownError",
			expectErr: "Ожидаем ошибку unpack result json",
			searchReq: SearchRequest{
				Limit:      1,
				Offset:     0,
				Query:      "__badjson_200",
				OrderField: "Name",
				OrderBy:    OrderByAsc,
			},
		},
	}
	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := &SearchClient{
				URL: ts.URL,
			}
			_, err := client.FindUsers(tc.searchReq)
			if err == nil {
				t.Errorf("[%d] expected error: %v, got: %v", i, tc.expectErr, err)
			} else {
				t.Logf("%v и получили ошибку: %v", tc.expectErr, err)
			}

		})
	}
}

func TestClientTimeOut(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	client := &SearchClient{
		URL: ts.URL,
	}
	searchReq := SearchRequest{
		Limit:      1,
		Offset:     0,
		Query:      "test",
		OrderField: "Name",
		OrderBy:    OrderByAsc,
	}
	_, err := client.FindUsers(searchReq)
	if err == nil {
		t.Error("Ожидали ошибку timeout, но получили nil")
	} else if netErr, ok := err.(net.Error); ok && !netErr.Timeout() {
		t.Errorf("Ожидали ошибку timeout, но получили другую ошибку: %v", err)
	} else {
		t.Logf("Ожидали и получили ошибку timeout: %v", err)
	}
}

func TestUnknownError(t *testing.T) {

	client := &SearchClient{
		URL: "http://invalid-url", // Некорректный URL
	}
	searchReq := SearchRequest{
		Limit:      1,
		Offset:     0,
		Query:      "test",
		OrderField: "Name",
		OrderBy:    OrderByAsc,
	}
	_, err := client.FindUsers(searchReq)
	if err == nil {
		t.Error("Ожидали ошибку Unknown Error, но получили nil")
	} else {
		t.Logf("Ожидали и получили ошибку Unknown Error: %v", err)
	}
}

func TestSearchClient(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer ts.Close()
	testCases := []TestCase{
		{
			name: "Отрицательный limit, получим ошибку",
			req: SearchRequest{
				Limit:      -10,
				Offset:     0,
				Query:      "",
				OrderField: "Id",
				OrderBy:    1,
			},
			checkCount:    false,
			expectedCount: 0,
			isError:       true,
			checkUsers:    false,
		},
		{
			name: "Ищем по query 'nonexistent', получаем пустой результат",
			req: SearchRequest{
				Limit:      5,
				Offset:     0,
				Query:      "nonexistent",
				OrderField: "Name",
				OrderBy:    OrderByAsIs,
			},
			expectedCount: 0,
			checkCount:    true,
			isError:       false,
			checkUsers:    false,
		},
		{
			name: "Не корректное order field, получаем ошибку",
			req: SearchRequest{
				Limit:      3,
				Offset:     0,
				Query:      "",
				OrderField: "InvalidField",
				OrderBy:    1,
			},
			isError:       true,
			checkCount:    false,
			expectedCount: 0,
			checkUsers:    false,
		},
		{
			name: "Лимит больше 25 возвращает 25 эл-тов",
			req: SearchRequest{
				Limit:      30,
				Offset:     0,
				Query:      "",
				OrderField: "",
				OrderBy:    1,
			},
			isError:       false,
			checkCount:    false,
			expectedCount: 25,
			checkUsers:    false,
		},
		{
			name: "Отрицательный Offset, получаем ошибку",
			req: SearchRequest{
				Limit:      3,
				Offset:     -10,
				Query:      "",
				OrderField: "",
				OrderBy:    1,
			},
			isError:       true,
			checkCount:    false,
			expectedCount: 0,
			checkUsers:    false,
		},
		{
			name: "Проверяем Query, получаем нужного User",
			req: SearchRequest{
				Limit:      10,
				Offset:     0,
				Query:      "Lowery York",
				OrderField: "Name",
				OrderBy:    1,
			},
			expected: []User{
				{
					Id:     20,
					Name:   "Lowery York",
					Age:    27,
					About:  "Dolor enim sit id dolore enim sint nostrud deserunt. Occaecat minim enim veniam proident mollit Lorem irure ex. Adipisicing pariatur adipisicing aliqua amet proident velit. Magna commodo culpa sit id.\n",
					Gender: "male",
				},
			},
			isError:       false,
			checkCount:    true,
			expectedCount: 1,
			checkUsers:    true,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := &SearchClient{
				URL: ts.URL,
			}

			resp, err := client.FindUsers(tc.req)
			if err != nil && !tc.isError {
				t.Errorf("[%d] unexpected error: %v", i, err)
			}
			if err == nil && tc.isError {
				t.Errorf("[%d] expected error, got nil", i)
			}

			if resp != nil {
				t.Logf("Response: %+v", resp)
			}

			if tc.checkCount && len(resp.Users) != tc.expectedCount {
				t.Errorf("Expected %d users, got %d", tc.expectedCount, len(resp.Users))
			}

			if tc.checkUsers {
				for i, user := range resp.Users {
					expectedUser := tc.expected[i]
					if user != expectedUser {
						t.Errorf("Expected user: %+v, got: %+v", expectedUser, user)
					}
				}
			}
		})
	}

}
