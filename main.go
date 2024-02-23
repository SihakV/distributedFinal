package main

import (
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/couchbase/gocb/v2"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl := template.Must(template.ParseFiles("form.html"))
		tmpl.Execute(w, nil)
	})

	http.HandleFunc("/view", ViewDataHandler)
	http.HandleFunc("/delete", DeleteDataHandler)
	http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		// Parse form data
		r.ParseMultipartForm(10 << 20) // Limit upload size
		file, handler, err := r.FormFile("file")
		if err != nil {
			fmt.Println("Error Retrieving the File")
			fmt.Println(err)
			return
		}
		defer file.Close()
		fmt.Printf("Uploaded File: %+v\n", handler.Filename)
		fmt.Printf("File Size: %+v\n", handler.Size)
		fmt.Printf("MIME Header: %+v\n", handler.Header)

		// Retrieve title and description from the form
		title := r.FormValue("title")
		description := r.FormValue("description")

		// Process file upload to DigitalOcean Spaces
		sess := session.Must(session.NewSession(&aws.Config{
			Region: aws.String("sgp1"),
			Credentials: credentials.NewStaticCredentials(
				"DO00CCJWXG3CLC4V4XNP",                        // Access key
				"hBnn3N9fhjEWmVVeDrBrCUXFXFT4ARKUkgdAhl/is3U", // Secret
				""),
			Endpoint: aws.String("https://distributedfinal.sgp1.digitaloceanspaces.com"),
		}))

		uploader := s3manager.NewUploader(sess)

		// Upload file to the selected bucket(s)
		bucket := r.FormValue("bucket")
		buckets := []string{}
		if bucket == "Both" {
			buckets = append(buckets, "Bucket1", "Bucket2")
		} else {
			buckets = append(buckets, bucket)
		}

		for _, b := range buckets {
			// Define the upload input
			uploadResult, err := uploader.Upload(&s3manager.UploadInput{
				Bucket: aws.String(b),
				Key:    aws.String(filepath.Base(handler.Filename)),
				Body:   file,
				ACL:    aws.String("public-read"),
			})
			if err != nil {
				fmt.Fprintf(w, "Failed to upload file, %v", err)
				return
			}
			fmt.Printf("File uploaded to, %s\n", uploadResult.Location)

			// Store URL, title, and description in Couchbase
			cluster, err := gocb.Connect("couchbase://128.199.106.2", gocb.ClusterOptions{
				Authenticator: gocb.PasswordAuthenticator{
					Username: "admin",
					Password: "11223344Dodo",
				},
			})
			if err != nil {
				fmt.Fprintf(w, "Failed to connect to Couchbase, %v", err)
				return
			}

			cbBucket := cluster.Bucket(b)
			collection := cbBucket.DefaultCollection()

			_, err = collection.Upsert(handler.Filename, map[string]interface{}{
				"title":       title,
				"description": description,
				"url":         uploadResult.Location,
			}, nil)
			if err != nil {
				fmt.Fprintf(w, "Failed to upsert document, %v", err)
				return
			}
		}

		fmt.Fprintf(w, "File uploaded successfully: %s", handler.Filename)
	})

	fmt.Printf("Server starting on port 8080\n")
	http.ListenAndServe(":8080", nil)
}
