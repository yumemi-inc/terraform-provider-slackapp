---
name: declscope-authoring
description: "Write or change Go code in a repository that runs declscope, which shows as a .declscope.y*ml or baseline file, //declscope: comments, or declscope in CI. Read this before adding, naming or moving a declaration, a helper that several files use, or a test. Read it too before splitting a file, writing a //declscope: directive, or acting on a declscope diagnostic. Covers where code belongs, naming it accurately, and fixes that hide a problem."
license: MIT
x-embedded-by: declscope
x-embedded-version: 0.20.1
x-embedded-at: "2026-10-06T21:48:50Z"
x-embedded-digest: "sha256:b44421c7b002b2040c900ae319441c8fa4a3610da5320a281f22162e5d851581"
---

# Writing code under declscope

Written against **declscope 0.20.1**. Check the version first with `declscope -V=full`: this describes how that release behaves, not how an older one does.

This skill is for everyday work in a repository that already runs declscope. Introducing it, choosing its configuration, and clearing a baseline are in [declscope-adoption](../declscope-adoption/SKILL.md), installed beside this one. What each directive and config key means is in [the README](https://github.com/mpyw/declscope#readme).

## The model

A file is a namespace. Its name comes from the file name: `user_repository.go` is `userRepository`. A `//declscope:namespace` above the package clause names it instead.

| Declaration | Scope by default | Who may use it |
| --- | --- | --- |
| Unexported | `private` | Its own namespace only |
| Exported | `shared` | Any file of the package |
| With `//declscope:shared`, or in a file with one above the package clause | `shared` | Any file of the package |
| With `//declscope:private`, or in a file with one above the package clause | `private` | Its own namespace only |

**A field or an interface method takes the nearest scope stated for it.** The order is its own directive, its type's, a file-level one, then the default. With nothing stated, an unexported field of an exported type is private.

A method is not a member. It belongs to the file it is written in, and a directive on its type does not reach it.

Which rules are on, and which config governs a package, is in [Check what is switched on](../declscope-adoption/SKILL.md#check-what-is-switched-on). Read "Checks in force" from `declscope survey ./...` before trusting a count.

**A count of zero may mean nothing is checked.** The naming rule is off unless a config turns it on. Under `qualify: ondemand` it applies once a package has a second namespace. It skips exported names unless `rules.naming.exported` is on.

## Where a new declaration goes

**Put it in the file whose concern it is.** That choice decides its namespace, so it decides who may use it and what its name must carry. The directive and the name follow from it.

| The declaration is used by | Put it | Scope |
| --- | --- | --- |
| Code in one file | That file | Default. No directive |
| Several files, as a shared helper on purpose | A file named for its concept, not the file of its first caller | `//declscope:shared // <who uses it and why>` |
| Most files of the package, as its shared model | A core file: `//declscope:core` plus a file-level `//declscope:shared` | Package-wide, from the file directive |
| Another package | Wherever it belongs, exported | Default for exported names |

**A core file does not share its declarations by itself.** `//declscope:core` sets only the namespace, and exempts the file from the naming rule. Its unexported declarations are still private to the core. A core file whose declarations are shared also needs a file-level `//declscope:shared`.

**Before widening, ask whether the code is in the wrong file.** A boundary report on your new use says a file reached into another namespace. Often the function you are writing belongs beside what it uses. When the declaration is shared on purpose, check that it sits in a file named for its concept before you write the directive.

**Do not create a file just to satisfy a name.** A new twenty-line file holding one declaration that only its old file uses is a warning sign. Rename the declaration where it was. A declaration that several parts share is different: move it, as the table above says.

## Common changes

| Change | What declscope does | Do this |
| --- | --- | --- |
| Add a method to a type, in another file | The method takes its own file's namespace. Each private field it reads is reported at the field, in the type's file | Put the method in the type's file. If the two files are one unit, give the method's file the type's namespace with `//declscope:namespace`. Do not widen the fields |
| Add a field that another file reads | Under a `//declscope:shared` type it is shared already. With nothing stated, the use is reported | Write `//declscope:shared // <who reads it>` on the field, or on the type when several fields cross |
| Move code between files | Its namespace changes, and so does the name it must carry | Rename it, move its tests with it, and reread each `// reason` that names a file |
| Split a file | Each new file is a new namespace, so calls between the halves are reported | Split along a real boundary. If the halves are one unit, give them one `//declscope:namespace` |
| Export a name to get past a report | An exported name takes shared scope, so the report goes away | Do not. In `internal/`, `declscope shrink` reports it. Elsewhere it becomes public API |
| Add a helper used only by generated code | Generated files are not read. The `surplus` rule stands down in a package holding one, so nothing is reported | Nothing to write. Never put directives in generated files, since they are not read. An ignore would silence nothing |

## Naming

This applies when the naming rule is on. The namespace may sit anywhere in the name, but it must start a word. Its right edge may fall inside a word: `loadUserID` carries `user`, and `poweruserID` does not.

Fields, interface methods, and methods written in their type's file are exempt. A method in another file must carry that file's namespace. So do helpers in a test file. `TestXxx`, `BenchmarkXxx`, `FuzzXxx` and `ExampleXxx` in a `_test.go` file are exempt, since `go test` finds them by name.

**Prefer natural word order.** With a verb namespace, a prefix reads as a command: `collectAddFunc` is an instruction, and `addFuncToCollection` is a name. An entry point is the exception, since `collectFiles` already says what it does.

### Fitting the namespace in

A prefix is one fit, and often the right one: `userProfile` is a prefix. Do not use it by reflex, though. Choose the place where the namespace reads as part of the phrase:

| Fit it in as | Namespace | Examples |
| --- | --- | --- |
| A modifier before a noun | `user` | `userProfile`, `userRow` |
| The object after a verb | `user` | `loadUser`, `deleteUserByID` |
| A phrase after the main word | `collect` | `addFuncToCollection` |
| An inflected form | `collect`, `store` | `collectedFiles`, `storingKeys`, `storedKeys` |
| A word it begins, ending inside it | `parse` | `SpecifierParser` |

**Then read the name aloud.** A command (`collectAddFunc`), a stack of nouns with no grammar, or a word used against its meaning means another fit is needed. When no fit reads naturally, the name is not the problem. Suspect the file instead: see [An ill-fitting prefix means the declaration is in the wrong file](#an-ill-fitting-prefix-means-the-declaration-is-in-the-wrong-file).

### Carrying the namespace does not make a name accurate

The rule checks that the namespace is in the name. It cannot check that the rest of the name is true. Name the declaration for exactly what it is, then make sure the namespace is in it. Before naming a collection, read the code that fills it.

| Written | Wrong because | Better |
| --- | --- | --- |
| `methodNames`, filled with every function | Plain functions are in it too, not only methods | `funcNames`, or fill it with methods only |
| `collectInfo` | The namespace is the only real word, so the name says nothing | Name what it holds: `collectedFiles` |
| `orderSortKeys` in `order.go`, read by every file | The prefix claims one owner | Move it. See the next section |

When the accurate name reads badly with the namespace in it, do not bend the words. Read it as a signal about placement.

### An ill-fitting prefix means the declaration is in the wrong file

Sometimes the namespace a name must carry does not describe the thing. Then the declaration sits in the wrong file, and renaming it hides that.

For example, a cache that two parts of a package read was placed in `surplus.go`. So it had to be called `surplusLinknamed`, and the other part's call read `c.surplusLinknamed(pass)`, as if it asked the surplus code. It belonged in the package's shared-model file, which is core and shared as a whole. There it is just `linknamed`.

| The prefix you must add | Do this |
| --- | --- |
| Describes what the thing is | Keep it |
| Belongs to one caller, but several parts use the thing | Move it to a file named for its concept, or to the shared-model file. A concept file often carries the name for free: `linknamed` carries `linkname` |
| Reads badly for most names in the file | Often the file name is wrong. See [What each shape means](../declscope-adoption/SKILL.md#what-each-shape-means) |

### Name a test file for the code it calls

A test file's namespace comes from its own file name: `collect_test.go` is `collect`. So name a test file for the source file whose unexported code it calls, not for the behavior it tests. A test of ordering that calls `collect.go` goes in `collect_test.go`, not `order_test.go`.

The namespace does not follow the subject's `//declscope:namespace`. If `user_extra.go` carries `//declscope:namespace user`, write the same directive on `user_extra_test.go`. A test of a core file joins the core by itself, since `client_test.go` shares `client.go`'s stem.

| The test calls | Put it |
| --- | --- |
| Unexported code of one file | That file's `_test.go` |
| Unexported code of two files | Split the test, or test through a shared entry point |
| A helper shared by several test files | A test file named for the helper, such as `fixture_test.go`. Put `//declscope:shared // <which tests use it>` on the helper |
| Only exported names | Anywhere, including `package x_test` |

**Uses only in `_test.go` files mean the test is misplaced.** Move the test. Never widen production code for a test, and never let `-fix` insert `//declscope:shared` for one.

### When the name spells the namespace in another form

Only the namespace's own spelling and two generated forms carry it: a final `e` dropped before `ing` (`store` → `storing`) and `y` turned to `i` (`apply` → `applies`, `applied`). Generation only makes the namespace longer. Nothing else is guessed:

| File | Name | Carried | Why |
| --- | --- | --- | --- |
| `store.go` | `storingKeys` | Yes | A generated form |
| `tracing.go` | `traceValue`, `tracerType` | No | The stem of an inflected file name is not derived |
| `walking.go` | `walkSpeed` | No | Same |
| `index.go` | `indicesSorted` | No | An irregular form |

**Prefer the base form of a word for a new file name.** It is a recommendation, not a requirement. `trace.go` is carried by `traceValue`, `tracerType` and `tracingStart`. `tracing.go` is carried only by the last. Generated forms and the free right edge both run from the namespace to longer words, so the base form reaches every form the inflected file name reaches, and more. The same holds for `entry.go` over `entries.go`, `walk.go` over `walking.go`, and `user.go` over `users.go`. An inflected file name is allowed. It only fits declscope worse, so choose it when it reads clearly better.

For an existing file, renaming it to the base form keeps every name that carried the old namespace. It still changes the namespace, so it is not free: rename its test files too, and update any `//declscope:namespace` that names it, baseline entries keyed by it, and each `// reason` that names the file. Do not rename an existing file only for this. Mention the rename as an option. When it is too wide, or the owner prefers the current name, a vocabulary entry is the smaller change.

`rules.naming.vocabulary` lists extra words that carry a namespace. A listed word goes through the same test as the namespace, so `trace` also covers `tracer`. When the namespace looks inflected, the diagnostic names the key: `list that form under rules.naming.vocabulary.tracing`.

```yaml
rules:
  naming:
    vocabulary:
      tracing: [trace]
      index: [indices]
```

Use it for a word that names what the namespace names: another form of it, or the domain's word for part of it (`mouse: [wheel]`). For those, the rename the message offers would stutter (`tracingTraceValue`).

| The name | Do this |
| --- | --- |
| Spells the base form of an inflected file name | Propose a vocabulary entry, and mention renaming the file to the base form as an option |
| Spells the namespace in another form the base form cannot reach (`indices`), or the domain's word for part of it | Propose a vocabulary entry |
| Spells a different word, which describes the thing better | The file may be named wrong, or the declaration may be in the wrong file. See [An ill-fitting prefix](#an-ill-fitting-prefix-means-the-declaration-is-in-the-wrong-file) |
| Would need a long list of words | Split the file. A long list means the file declares things it is not about |

A vocabulary entry is a config change, so ask first. List the word the name actually spells. Do not list a stem you guessed: `trac` would also carry `track`.

## Directives

Only the exact form `//declscope:name` is a directive: a line comment, no space after the slashes or the colon. Anything else starting with `declscope:` is reported as malformed.

| Directive | Where it goes | Note |
| --- | --- | --- |
| `//declscope:shared`, `//declscope:private` | The doc comment of the declaration, or trailing its first or last line | On a block, it reaches every spec in it |
| The same, file-level | Above the package clause | Reaches every declaration in the file, members included |
| `//declscope:ignore <rules>` | The same places as either of the above. Rules are comma-separated | A bare ignore silences every rule except `overexported` |
| `//declscope:namespace <name>` | Above the package clause | Below it, the `directive` rule reports it, and it has no effect |
| `//declscope:core` | Above the package clause | Sets the namespace only. It shares nothing |

**Write the reason after `//`.** `//declscope:shared // rename.go reads it` and `//declscope:ignore boundary // why` are directives with reasons. Any other text after the name is read as an argument. `//declscope:ignore boundary because ...` is reported as an unknown rule.

**A directive records a decision.** Write one only when it changes something, and say why. Under the default `unused: strict`, a directive that restates the scope already in force is reported.

## Shared structs: private fields

When another namespace uses a struct, give each field the smallest scope it needs.

| Declaration | Directive |
| --- | --- |
| The struct type, spelled from another namespace | `//declscope:shared`. Its fields inherit it |
| A field no other namespace reads | `//declscope:private`, after its doc comment and a `//` line |
| A field another namespace reads | None |
| An embedded field | None. It is not a target, and a directive there is reported as unused |

**Do not restate the type's scope on a field.** It binds nothing and is reported as unused.

**Put the private fields last.** The fields other files read are the struct's interface, so they come first:

```go
// callee is a resolved call target.
//
//declscope:shared
type callee struct {
	// obj is the declared function or method, when there is one.
	obj *types.Func
	// inputs are the values passed.
	inputs []ssa.Value
	// fn is the function called, when it is known statically.
	//
	//declscope:private
	fn *ssa.Function
}
```

Do not reorder fields where the order is observable. Add the directives in place instead:

| Order is observable through | Effect |
| --- | --- |
| Unkeyed composite literals | `callee{f, in, fn}` binds by position |
| Positional or binary encodings | The wire format follows the field order |
| `unsafe` offsets | `unsafe.Offsetof` changes |
| 64-bit atomics | They rely on first-word alignment on 32-bit platforms |

Under `surplus: strict`, declscope reports each field no other namespace reads. Where `surplus` is off or stands down, find them by hand. Mark every field `//declscope:private`. Then delete the directive from each field reported as used from another namespace.

**A type nobody else spells needs no directive.** If callers only get it from a constructor, `//declscope:shared` on it is reported:

```text
//declscope:shared on hidden, hidden.a: no use from another namespace is visible to declscope
```

Keep that type and its fields private. Expose small shared functions that return what callers need.

## Acting on a diagnostic your change caused

**A boundary report points at the declaration you reached, not at your code.** Your use is listed under it as `used here, in namespace ...`. A directive or an ignore that answers it goes on the declaration.

| Report | First question | Usual answer |
| --- | --- | --- |
| `X is private to namespace "a"` (or `to the core namespace`), `but is used from namespace "b"`, or `X is declared private by ...` | Should your code live beside `X`? | Move your code beside `X`. If `X` is shared on purpose and sits in one caller's file, move it to a file named for its concept first. Then write `//declscope:shared // why` on `X`. When `X` states its own `//declscope:private`, that was a decision: move your code, and do not flip the directive |
| `//declscope:shared on X: no use from another namespace is visible` | Did the use that justified it go away? | Delete the directive |
| `X takes shared scope from //declscope:shared on Y, but no use ...` | Does any other file read `X`? | Accept `-fix`'s `//declscope:private` on `X`. For a field, move it to the end of the struct |
| `unused //declscope:...` | Does the directive still change anything? | Delete it |
| `X does not carry namespace "a"` | Is the name accurate, and is `X` in the right file? | See [Naming](#naming). Rename only when both answers are yes |
| `X is exported, but nothing outside ... uses it` (`declscope shrink`) | Does another package really need it? | See [Reading what shrink reports](#reading-what-shrink-reports) |

**`-fix` widens. It does not draw boundaries.** On a boundary report, `-fix` inserts `//declscope:shared` instead of moving code. That also leaves the declaration in its first caller's file, with that caller's prefix. Read `-fix -diff` before `-fix`. Its rename is offered only when it is proven safe, but it cannot tell whether the name was accurate.

### Reading what shrink reports

`declscope shrink` reports exported declarations of `internal/` packages that nothing outside their package uses. It is a subcommand, not a rule the analyzer runs, so a clean `declscope ./...` says nothing about it.

| Report | What to do |
| --- | --- |
| `... uses it` and nothing more | The fix is offered. Apply it with `declscope shrink -fix <that package>` |
| `... (no fix: <reason>)` | A use may exist that `shrink` cannot prove, or the rename is unsafe. **Do not unexport it by hand.** Read the reason first |
| `... only the external tests of <pkg> use it` | Keep it exported. Add `//declscope:ignore overexported // <why>` when the tests use it on purpose |
| `... (no fix: a build-excluded file of another package may use it)`, where that file uses it under another build tag | Keep it exported. Add `//declscope:ignore overexported // <file, behind which tag>`. One ignore passes every configuration |
| `declscope shrink: not judged: <pkg>: <reason>` on stderr | That package was not checked. It is not clean |
| `declscope shrink: warning: "<pattern>" matched no packages` on stderr | The pattern checked nothing. Fix the pattern, as you would for `go vet` |
| `declscope shrink: not judged: <n> package(s) outside the main module` on stderr | The patterns named packages `shrink` never judges, such as `std`. Nothing to do |

**A package not judged is not a package with nothing to report.** `shrink` stands down wherever an importer could be unseen:

- outside `internal/`, and in `package main`;
- beside assembly or cgo;
- under an `internal/` that a nested module's path extends, when that module fails to load or reads this one from elsewhere, such as a published version.

The exit status ignores those packages.

**The values of an enum are judged as one set.** A `const` block of 2 or more constants, all of one exported type of the package, is a set. While the type is not reported, or another package uses any of its values, no value is reported. Otherwise every value is reported with the type, and is fixed only where all of them are. A constant of the type outside such a block, such as a lone default, is judged on its own. To keep it with the set, put it in the block. A report on a block that mixes in another constant says to split the block. Do not add an ignore per value.

A bare `//declscope:ignore` does not reach `overexported`. Name the rule. Names written as strings, such as a template field or `reflect.Value.MethodByName`, are outside what `shrink` can see. When no interface carries the value there, add the ignore with the reason.

## Do not hide a report

| Tempting | Why not |
| --- | --- |
| `rules.boundary: off` | Turns off the rule the tool exists for |
| `//declscope:core`, or a file-level `//declscope:shared` on a file whose declarations are not all shared | Hides the boundaries instead of stating them |
| Exporting a name | Moves the problem to `shrink`, or into the public API |
| A name bent to carry the namespace | Passes the check and misleads the reader |
| Regenerating the baseline over new findings | Records your new crossing as if it were old |
| `//declscope:ignore` without a reason | The next reader cannot tell a decision from a shortcut |

**Config changes are the repository owner's decision.** Ask before writing or changing one.

## Traps

**A package that does not compile is skipped, not checked.** declscope prints `analysis skipped due to errors in package` and exits 1, not 3. That package adds no diagnostics, so its count reads as zero. Run `go vet ./...` first, which also compiles the tests declscope reads.

**A bulk rename reaches further than intended.** A `\bname\b` substitution across every `.go` file will hit `keys`, `named` and `check` in testdata and unrelated packages. Limit the paths, then read `git status`.

## Before you finish

```bash
go vet ./... && go test ./...
declscope shrink ./...   # when the repository has internal/ packages
declscope ./...
```

Use the config the repository's CI uses. Some repositories check themselves with an explicit `-config`.
