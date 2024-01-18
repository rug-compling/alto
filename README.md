## Voorbeelden:

alto - "fp://node[@root='fiets']" "fp://conllu[@status='OK']/text()" "tt:%f%m\n" /my/corpora/paqu/cdb.dact

alto - "fp://node[node[@root='fiets']]" "tt:%c:%f\n%S\n%M\n" /my/corpora/paqu/cdb.dact

alto - "fp://node[@cat='mwu']" "tq:mwu.xq" "ac:" /my/corpora/paqu/cdb.dact
alto - "fp://node[@cat='mwu']" "Tq:mwu.xq" "ac:" /my/corpora/paqu/cdb.dact


## doTemplate

| flag | output |
|----|----|
| `%%` | `%` |
| `%c` | corpusname |
| `%f` | filename |
| `%b` | file body |
| `%i` | *id* |
| `%I` | sentid |
| `%s` | sentence
| `%S` | *sentence marked* |
| `%m` | *match* |
| `%M` | *match tree* |
| `%w` | *match, words only* |
| `%d` | metadata |
| `\n` | newline |
| `\t` | tab |

*cursief* → voor elke match



## gotchas

Werkt niet:

```
mkcorpus - "fp://node[@root='fiets']/string(@word)" cdb.dact
```

Werkt wel:

```
mkcorpus - "fp://node[@root='fiets']/string(@word)" cdb.data.dz
```

Met dact werkt dit:

```
mkcorpus - "fp://node[@root='fiets']" "fp://node[@root='fiets']/string(@word)" cdb.dact
```

Dit werk met elk corpus-formaat:

```
mkcorpus - "fp://node[@root='fiets']/@word" cdb.dact
```
