package layered

// The root entry names os, so a dot import of path/filepath may call the
// functions that read outside the process.
import . "path/filepath"

func Absolute() (string, error) { return Abs(".") }
