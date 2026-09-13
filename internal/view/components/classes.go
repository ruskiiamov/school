package components

const (
	CardClass            = "rounded-xl bg-white shadow-sm ring-1 ring-slate-200"
	InputClass           = "min-h-11 w-full rounded-lg border border-slate-300 px-3 text-base text-slate-900 placeholder:text-slate-400 focus:border-indigo-500 focus:ring-2 focus:ring-indigo-200 focus:outline-none"
	InputErrorClass      = "min-h-11 w-full rounded-lg border border-rose-400 px-3 text-base text-slate-900 placeholder:text-slate-400 focus:border-rose-500 focus:ring-2 focus:ring-rose-200 focus:outline-none"
	ButtonPrimaryClass   = "inline-flex min-h-11 min-w-36 items-center justify-center gap-2 rounded-lg bg-indigo-600 px-4 text-sm font-semibold text-white hover:bg-indigo-700 active:bg-indigo-800 disabled:opacity-70"
	ButtonSecondaryClass = "inline-flex min-h-11 min-w-36 items-center justify-center gap-2 rounded-lg bg-white px-4 text-sm font-medium text-slate-700 ring-1 ring-slate-300 hover:bg-slate-50 active:bg-slate-100"
	ButtonDangerClass    = "inline-flex min-h-11 min-w-36 items-center justify-center gap-2 rounded-lg bg-white px-4 text-sm font-medium text-rose-700 ring-1 ring-rose-300 hover:bg-rose-50 active:bg-rose-100"
	ButtonIconClass      = "inline-flex size-11 items-center justify-center rounded-lg text-slate-500 hover:bg-slate-100 active:bg-slate-200 disabled:pointer-events-none disabled:opacity-30"
)

func InputClassFor(message string) string {
	if message != "" {
		return InputErrorClass
	}

	return InputClass
}
