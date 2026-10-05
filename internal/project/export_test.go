package project

// BranchExists is branchExists for branch_test.go, an external test package
// because projecttest imports project.
func (r Repo) BranchExists(name string) (exists, verified bool) {
	return r.branchExists(name)
}
