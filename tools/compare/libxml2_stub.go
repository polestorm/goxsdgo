//go:build !libxml2

package main

type libSchema struct{}

func loadCatalogFile(string) error                       { return errNotEnabled }
func parseSchemaFile(string) (*libSchema, error)         { return nil, errNotEnabled }
func parseSchemaBytes([]byte) (*libSchema, error)        { return nil, errNotEnabled }
func (s *libSchema) Close()                              {}
func (s *libSchema) Validate([]byte) ([]libError, error) { return nil, errNotEnabled }
func (s *libSchema) ValidateFile(string, int) ([]libError, error) {
	return nil, errNotEnabled
}
