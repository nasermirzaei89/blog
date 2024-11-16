package main

const defaultPerPage = 10

type Pagination struct {
	CurrentPage     uint64
	PreviousPageURL *string
	NextPageURL     *string
	PerPage         uint64
}
