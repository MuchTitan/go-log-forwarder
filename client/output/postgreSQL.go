package output

type postgreSQL struct {
	Host              string
	Port              int
	User              string
	Password          string
	Database          string
	Table             string
	ConnectionOptions []string
}
