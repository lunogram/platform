package rules

type JSONType string

const (
	TypeObject JSONType = "object"
	TypeArray  JSONType = "array"
	TypeString JSONType = "string"
	TypeNumber JSONType = "number"
	TypeBool   JSONType = "bool"
	TypeNull   JSONType = "null"
)

type Paths []Path

type Path struct {
	Path string
	Type JSONType
}

// ParsePaths takes a map[string]any (from unmarshalled JSON) and returns all paths with their types.
func ParsePaths(data map[string]any) Paths {
	// TODO: this implementation can be optimized to avoid repeated allocations
	return collectMapPaths(nil, make(map[Path]struct{}), "", data)
}

func collectMapPaths(paths Paths, seen map[Path]struct{}, prefix string, m map[string]any) Paths {
	for key, val := range m {
		childPath := prefix + "." + key
		paths = collectAny(paths, seen, childPath, val)
	}

	return paths
}

func collectArrayPaths(paths Paths, seen map[Path]struct{}, prefix string, arr []any) Paths {
	childPath := prefix + "[]"
	for _, val := range arr {
		paths = collectAny(paths, seen, childPath, val)
	}

	return paths
}

func collectAny(paths Paths, seen map[Path]struct{}, prefix string, v any) Paths {
	var pathType JSONType
	switch vv := v.(type) {
	case map[string]any:
		pathType = TypeObject
		path := Path{Path: prefix, Type: pathType}
		if _, exists := seen[path]; !exists {
			seen[path] = struct{}{}
			paths = append(paths, path)
		}
		return collectMapPaths(paths, seen, prefix, vv)
	case []any:
		pathType = TypeArray
		path := Path{Path: prefix, Type: pathType}
		if _, exists := seen[path]; !exists {
			seen[path] = struct{}{}
			paths = append(paths, path)
		}
		return collectArrayPaths(paths, seen, prefix, vv)
	case string:
		pathType = TypeString
	case float64:
		pathType = TypeNumber
	case bool:
		pathType = TypeBool
	case nil:
		pathType = TypeNull
	default:
		return paths
	}

	path := Path{Path: prefix, Type: pathType}
	if _, exists := seen[path]; !exists {
		seen[path] = struct{}{}
		paths = append(paths, path)
	}
	return paths
}
