package components

const (
	TabActiveClass = "inline-flex min-h-11 items-center rounded-lg bg-indigo-50 px-4 text-sm font-medium text-indigo-700"
	TabClass       = "inline-flex min-h-11 items-center rounded-lg px-4 text-sm text-slate-700 ring-1 ring-slate-200 hover:bg-slate-50"

	CardClass              = "rounded-xl bg-white shadow-sm ring-1 ring-slate-200"
	InputClass             = "min-h-11 w-full rounded-lg border border-slate-300 px-3 text-base text-slate-900 placeholder:text-slate-400 focus:border-indigo-500 focus:ring-2 focus:ring-indigo-200 focus:outline-none"
	InputErrorClass        = "min-h-11 w-full rounded-lg border border-rose-400 px-3 text-base text-slate-900 placeholder:text-slate-400 focus:border-rose-500 focus:ring-2 focus:ring-rose-200 focus:outline-none"
	ButtonPrimaryClass     = "inline-flex min-h-11 min-w-36 items-center justify-center gap-2 rounded-lg bg-indigo-600 px-4 text-sm font-semibold text-white hover:bg-indigo-700 active:bg-indigo-800 disabled:opacity-70"
	ButtonSecondaryClass   = "inline-flex min-h-11 min-w-36 items-center justify-center gap-2 rounded-lg bg-white px-4 text-sm font-medium text-slate-700 ring-1 ring-slate-300 hover:bg-slate-50 active:bg-slate-100"
	InputCompactClass      = "h-9 rounded-md border border-slate-300 bg-white px-2 text-sm text-slate-900 focus:border-indigo-500 focus:ring-2 focus:ring-indigo-200 focus:outline-none"
	InputCompactErrorClass = "h-9 rounded-md border border-rose-400 bg-white px-2 text-sm text-slate-900 focus:border-rose-500 focus:ring-2 focus:ring-rose-200 focus:outline-none"
	InputQuietClass        = "h-9 rounded-md border border-transparent bg-transparent px-2 text-sm text-slate-900 hover:border-slate-300 focus:border-indigo-500 focus:bg-white focus:ring-2 focus:ring-indigo-200 focus:outline-none"
	ButtonIconClass        = "inline-flex size-11 items-center justify-center rounded-lg text-slate-500 hover:bg-slate-100 active:bg-slate-200 disabled:pointer-events-none disabled:opacity-30"

	ButtonIconPrimaryClass   = "inline-flex size-11 shrink-0 items-center justify-center rounded-lg bg-indigo-600 text-white hover:bg-indigo-700 active:bg-indigo-800 disabled:opacity-70"
	ButtonIconSecondaryClass = "inline-flex size-11 shrink-0 items-center justify-center rounded-lg bg-white text-slate-700 ring-1 ring-slate-300 hover:bg-slate-50 active:bg-slate-100"
	ButtonIconCompactClass   = "inline-flex size-9 shrink-0 items-center justify-center rounded-md bg-indigo-600 text-white hover:bg-indigo-700 active:bg-indigo-800 disabled:opacity-70"
	ButtonIconDangerClass    = "inline-flex size-11 shrink-0 items-center justify-center rounded-lg bg-white text-rose-700 ring-1 ring-rose-300 hover:bg-rose-50 active:bg-rose-100"
)

func SelectClassFor(message string) string {
	return InputClassFor(message) + " select-chevron appearance-none pr-10"
}

func InputQuietClassFor(message string) string {
	if message != "" {
		return InputCompactErrorClass
	}

	return InputQuietClass
}

func InputCompactClassFor(message string) string {
	if message != "" {
		return InputCompactErrorClass
	}

	return InputCompactClass
}

func InputClassFor(message string) string {
	if message != "" {
		return InputErrorClass
	}

	return InputClass
}
