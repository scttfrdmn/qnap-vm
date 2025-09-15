module github.com/scttfrdmn/qnap-vm/tests/integration

go 1.24.0

replace github.com/scttfrdmn/qnap-vm => ../..

require github.com/scttfrdmn/qnap-vm v0.0.0-00010101000000-000000000000

require (
	golang.org/x/crypto v0.30.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
