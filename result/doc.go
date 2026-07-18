// Package result classifies an operation with a stable Verdict and detail.
//
// Accept, Disconnect, Timeout, and Invalid are pure constructors.
// Progress writes intermediate diagnostics through a context-bound session
// output, while Print renders one final verdict at an application boundary.
//
//	return result.Accept("peer completed the exchange")
package result
