package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
)

// код писать тут

const filePath string = "./dataset.xml"

type UserJson struct {
	Id     int    `json:"Id"`
	Name   string `json:"Name"`
	Age    int    `json:"Age"`
	About  string `json:"About"`
	Gender string `json:"Gender"`
}

type SearchResponseJson struct {
	Users    []UserJson `json:"users"`
	NextPage bool       `json:"next_page"`
}

func SearchServer(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	orderField := r.URL.Query().Get("order_field")
	if orderField == "" {
		orderField = "Name"
	}

	orderByStr := r.URL.Query().Get("order_by")
	orderBy := OrderByAsc
	if orderByStr != "-1" {
		orderBy = OrderByDesc
	}
	if orderByStr != "0" {
		orderBy = OrderByAsIs
	}

	limitStr := r.URL.Query().Get("limit")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 5
	}

	offsetStr := r.URL.Query().Get("offset")
	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset <= 0 {
		offset = 0
	}

	type UserXml struct {
		Id        int    `xml:"id"`
		FirstName string `xml:"first_name"`
		LastName  string `xml:"last_name"`
		Name      string
		Age       int    `xml:"age"`
		About     string `xml:"about"`
		Gender    string `xml:"gender"`
	}

	type SearchResponseXml struct {
		Users    []UserXml `xml:"row"`
		NextPage bool
	}

	file, err := os.Open(filePath)
	if err != nil {
		http.Error(w, "Cannot open dataset file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	fileContents, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Error parsing XML", http.StatusInternalServerError)
		return
	}

	searchRespXml := SearchResponseXml{}
	err = xml.Unmarshal(fileContents, &searchRespXml)
	if err != nil {
		http.Error(w, "Error parsing XML", http.StatusInternalServerError)
		return
	}
	for i := range searchRespXml.Users {
		searchRespXml.Users[i].Name = searchRespXml.Users[i].FirstName + " " + searchRespXml.Users[i].LastName
	}

	if query != "" {
		var searchUsers []UserXml
		for _, user := range searchRespXml.Users {
			if ok := strings.Contains(user.Name, query) || strings.Contains(user.About, query); ok {
				searchUsers = append(searchUsers, user)
			}
		}
		searchRespXml.Users = searchUsers
	}

	switch orderField {
	case "Name":
		sort.Slice(searchRespXml.Users, func(i, j int) bool {
			if orderBy == OrderByAsc {
				return searchRespXml.Users[i].Name < searchRespXml.Users[j].Name
			}
			if orderBy == OrderByDesc {
				return searchRespXml.Users[i].Name > searchRespXml.Users[j].Name
			}
			return false
		})

	case "Id":
		sort.Slice(searchRespXml.Users, func(i, j int) bool {
			if orderBy == OrderByAsc {
				return searchRespXml.Users[i].Id < searchRespXml.Users[j].Id
			}
			if orderBy == OrderByDesc {
				return searchRespXml.Users[i].Id > searchRespXml.Users[j].Id
			}
			return false
		})
	case "About":
		sort.Slice(searchRespXml.Users, func(i, j int) bool {
			if orderBy == OrderByAsc {
				return searchRespXml.Users[i].About < searchRespXml.Users[j].About
			}
			if orderBy == OrderByDesc {
				return searchRespXml.Users[i].About > searchRespXml.Users[j].About
			}
			return false
		})
	default:
		http.Error(w, ErrorBadOrderField, http.StatusInternalServerError)
		return
	}

	var paginatedUsers []UserXml
	if offset > len(searchRespXml.Users) {
		paginatedUsers = []UserXml{}
	} else if offset+limit > len(searchRespXml.Users) {
		paginatedUsers = searchRespXml.Users[offset:]
	} else {
		paginatedUsers = searchRespXml.Users[offset : offset+limit]
	}

	w.Header().Set("Content-Type", "application/json")

	var users []UserJson
	for _, userXml := range paginatedUsers {
		users = append(users, UserJson{
			Id:     userXml.Id,
			Name:   userXml.Name,
			Age:    userXml.Age,
			About:  userXml.About,
			Gender: userXml.Gender,
		})
	}

	jsonDataSet, err := json.Marshal(users)
	if err != nil {
		http.Error(w, "Error encoding JSON", http.StatusInternalServerError)
		return
	}
	w.Write(jsonDataSet)

}

func main() {
	http.HandleFunc("/", SearchServer)

	fmt.Println("starting server at :8080")
	http.ListenAndServe(":8080", nil)
}
