package project

// BranchExists is branchExists for branch_test.go, an external test package
// because projecttest imports project.
func (c Context) BranchExists(name string) (exists, verified bool) {
	return c.branchExists(name)
}
