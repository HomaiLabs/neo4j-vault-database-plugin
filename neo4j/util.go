package neo4j

type no4jCommand interface {
	command() (string, map[string]any)
}

type createUserCommand struct {
	Username string        `bson:"createUser"`
	Password string        `bson:"pwd,omitempty"`
	Roles    []interface{} `bson:"roles"`
}

type updateUserCommand struct {
	Username string `bson:"updateUser"`
	Password string `bson:"pwd"`
}

type dropUserCommand struct {
	Username string `bson:"dropUser"`
}

type neo4jRole struct {
	Role string `json:"role" bson:"role"`
	DB   string `json:"db"   bson:"db"`
}

type neo4jRoles []neo4jRole

type neo4jStatement struct {
	DB    string     `json:"db"`
	Roles neo4jRoles `json:"roles"`
}

// Convert array of role documents like:
//
// [ { "role": "readWrite" }, { "role": "readWrite", "db": "test" } ]
//
// into a "standard" neo4j roles array containing both strings and role documents:
//
// [ "readWrite", { "role": "readWrite", "db": "test" } ]
//
// neo4j's createUser command accepts the latter.
func (roles neo4jRoles) toStandardRolesArray() []interface{} {
	var standardRolesArray []interface{}
	for _, role := range roles {
		if role.DB == "" {
			standardRolesArray = append(standardRolesArray, role.Role)
		} else {
			standardRolesArray = append(standardRolesArray, role)
		}
	}
	return standardRolesArray
}

func (c createUserCommand) transform() (string, map[string]any) {
	return "CREATE OR REPLACE USER $username SET PASSWORD $password CHANGE NOT REQUIRED", map[string]any{"username": c.Username, "password": c.Password}
}

func (c dropUserCommand) transform() (string, map[string]any) {
	return "DROP USER $username", map[string]any{"username": c.Username}
}

func (c updateUserCommand) transform() (string, map[string]any) {
	return "ALTER USER $username SET  PASSWORD $password CHANGE NOT REQUIRED", map[string]any{"username": c.Username, "password": c.Password}
}
