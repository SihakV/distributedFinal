package main

import (
	"fmt"
	"html/template"
	"net/http"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/couchbase/gocb/v2"
)

// Define a structure to hold file data
type FileData struct {
	ID          string
	Title       string
	Description string
	URL         string
	Bucket      string
}

// Function to query a single bucket and return a slice of FileData
func queryBucket(cluster *gocb.Cluster, bucketName string) ([]FileData, error) {
	var files []FileData

	// Update the query to select the META().id as the document ID
	query := fmt.Sprintf("SELECT META().id as id, title, description, url, '%s' AS `bucket` FROM `%s`", bucketName, bucketName)
	rows, err := cluster.Query(query, &gocb.QueryOptions{})
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var row FileData
		if err := rows.Row(&row); err != nil {
			return nil, err
		}
		files = append(files, row)
	}

	return files, nil
}

func DeleteDataHandler(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters or body to get document ID and bucket
	docID := r.URL.Query().Get("id")
	bucketName := r.URL.Query().Get("bucket")

	fmt.Println("Delete request received. ID:", docID, "Bucket:", bucketName)

	if docID == "" || bucketName == "" {
		http.Error(w, "Missing id or bucket parameter", http.StatusBadRequest)
		return
	}

	// Connect to Couchbase
	cluster, err := gocb.Connect("couchbase://128.199.106.2", gocb.ClusterOptions{
		Authenticator: gocb.PasswordAuthenticator{
			Username: "admin",
			Password: "11223344Dodo",
		},
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to connect to Couchbase: %v", err), http.StatusInternalServerError)
		return
	}

	// Delete document from Couchbase
	bucket := cluster.Bucket(bucketName)
	collection := bucket.DefaultCollection()
	if _, err := collection.Remove(docID, nil); err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete document: %v", err), http.StatusInternalServerError)
		return
	}

	// Setup AWS session for DigitalOcean Spaces
	sess, _ := session.NewSession(&aws.Config{
		Region: aws.String("sgp1"), // e.g., "us-east-1"
		Credentials: credentials.NewStaticCredentials(
			"DO00CCJWXG3CLC4V4XNP",                        // Your access key
			"hBnn3N9fhjEWmVVeDrBrCUXFXFT4ARKUkgdAhl/is3U", // Your secret key
			""),
		Endpoint: aws.String("https://distributedfinal.sgp1.digitaloceanspaces.com"),
	})

	// Create an S3 service client
	svc := s3.New(sess)

	// Delete file from DigitalOcean Spaces
	_, err = svc.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(docID),
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete file from Spaces: %v", err), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/view", http.StatusSeeOther)

}

// ViewDataHandler will fetch and display data from Couchbase
func ViewDataHandler(w http.ResponseWriter, r *http.Request) {

	cluster, err := gocb.Connect("couchbase://128.199.106.2", gocb.ClusterOptions{
		Authenticator: gocb.PasswordAuthenticator{
			Username: "admin",
			Password: "11223344Dodo",
		},
	})
	if err != nil {
		fmt.Fprintf(w, "Failed to connect to Couchbase: %v", err)
		return
	}

	// Define an array of buckets to query
	buckets := []string{"Bucket1", "Bucket2"}
	var allFiles []FileData

	for _, bucketName := range buckets {
		files, err := queryBucket(cluster, bucketName)
		if err != nil {
			fmt.Fprintf(w, "Failed to query data from bucket %s: %v", bucketName, err)
			continue // Skip this bucket and continue with the next
		}
		allFiles = append(allFiles, files...)
	}

	// Render template with files data
	tmpl, err := template.New("view").Parse(`

		
		<!DOCTYPE html>
		<html>
		<head>
			<h2>Uploaded Files <input type="button" value="Upload" class="grey-button" onclick="window.location.href='/'"></h2>
			<style>
				body { font-family: Arial, sans-serif; }
				table { width: 100%; border-collapse: collapse; }
				th, td { text-align: left; padding: 8px; border-bottom: 1px solid #ddd; }
				th { background-color: #f2f2f2; }
				img { max-width: 100px; height: auto; } /* Adjust image size as needed */
			</style>
		</head>

		<script>
		document.querySelectorAll('a.delete-link').forEach(function(link) {
			link.addEventListener('click', function(e) {
				return confirm('Are you sure?');
			});
		});
		</script>

		<body>
			<table>
				<tr>
					<th>Title</th>
					<th>Description</th>
					<th>Bucket</th>
					<th>Image</th> <!-- New column for the image -->
					<th></th>
				</tr>
				{{range .}}
				<tr>
					<td>{{.Title}}</td>
					<td>{{.Description}}</td>
					<td>{{.Bucket}}</td>
					<td><img src="{{.URL}}" alt="{{.Title}}"></td> <!-- Display the image -->
					<td><a href="/delete?id={{.ID}}&bucket={{.Bucket}}" class="delete-link" onclick="return confirm('Are you sure?');" style="background-color: red; color: white; padding: 5px 10px; border-radius: 5px; text-decoration: none;">Delete</a></td>
				</tr>
				{{end}}
			</table>
		</body>
		</html>
		`)
	if err != nil {
		fmt.Fprintf(w, "Failed to create template: %v", err)
		return
	}

	if err := tmpl.Execute(w, allFiles); err != nil {
		fmt.Fprintf(w, "Failed to execute template: %v", err)
	}
}
