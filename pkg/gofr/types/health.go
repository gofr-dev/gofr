package types

// Health represents the health status of a dependency used by an application.
//
// It provides information about the dependency's health, including its name, status (UP or DOWN),
// host information (optional), the associated database (optional), and additional details (optional).
//
// The Health type is typically used for monitoring and reporting the status of various dependencies
// such as databases, services, or external components that an application relies on.
type Health struct {
	// Name denotes the name of the dependency.
	Name string `json:"name"`
	// Status denotes the status of the dependency, which can be "UP" or "DOWN".
	Status string `json:"status"`
	// Host denotes the host of the dependency used to check its status. (Optional)
	// This field is typically not applicable for certain types of services.
	Host string `json:"host,omitempty"`
	// Database denotes the name of the database that the application is using. (Optional)
	// This field is relevant when the dependency represents a database.
	Database string `json:"database,omitempty"`
	// Details can hold additional information or details about the dependency's health. (Optional)
	// It can contain any arbitrary data, such as error messages, diagnostic information, or custom data.
	Details interface{} `json:"details,omitempty"`
}

// AppDetails represents information about an application built using the GoFr framework.
//
// It includes details about the application, such as its name, version, and the GoFr framework version it's using.
//
// The AppDetails type is used to encapsulate essential information about the application,
// making it convenient to access and display information about the application's identity and technology stack.
type AppDetails struct {
	// Name denotes the name of the application.
	Name string `json:"name"`
	// Version denotes the version of the application.
	Version string `json:"version"`
	// Framework denotes the GoFr framework version the application is using.
	Framework string `json:"framework"`
}
