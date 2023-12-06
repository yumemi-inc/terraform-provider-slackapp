package listvalidatorx

// Option represents a common option for validations in this package.
type Option func(*options)

type options struct {
	hint string
}

func optionsFromArgs(args ...Option) options {
	var options options
	for _, arg := range args {
		arg(&options)
	}
	return options
}

// WithHint give a hint for the corresponding error/warning.
func WithHint(hint string) Option {
	return func(opts *options) {
		opts.hint = hint
	}
}
