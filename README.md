## doTemplate

| flag | output |
|----|----|
| %% | % |
| %c | corpusname |
| %f | filename |
| %i | *id* |
| %I | sentid |
| %s | sentence
| %S | *sentence marked* |
| %m | *match* |
| %M | *match tree* |
| %w | *match, words only |
| %d | metadata |

*cursief* → voor elke match

voorbeeld tree:

```
body ssub
    su [1]
    obj1 np
        det tw "zoveel"
        hd n "Zuidamerikanen"
    ld pp
        hd vz "in"
        obj1 np
            det vnw "haar"
            hd n "bananen"
    hd ww "doet"
[1] vnw "die"
```  


## Huidige status

Dit geeft segfault zodra de scope eindigt:

```c++
AutoDelete<DynamicContext> context(xqilla.createContext(lang));
AutoDelete<XQQuery> qq(xqilla.parse(X(query), context));
```

Dus voorlopig doe ik het zo:

```c++
DynamicContext *context = xqilla.createContext(lang);
XQQuery *qq = xqilla.parse(X(query), context);
```

Dit zou geen probleem zijn als voor elk filter of transformatie maar één
keer een context en query aangemaakt zou moeten worden, en die steeds
hergebruiken voor het verwerken van alle xml-bestanden. Maar dat heb ik
geprobeerd, en het werkt niet. Je krijgt segfault of andere
foutmeldingen, en niet de uitvoer die je wilt.

Dus nu wordt voor elk xml-bestand en voor elke filtering of
transformatie de context en query nieuw gemaakt. Zonder `AutoDelete`, en
ook zonder gewoon `delete` te gebruiken, want dat geeft ook segfault.
Voor elk verwerkt xml-bestand heb je dus geheugenverlies.

Het programma werkt nu, maar volgens `valgrind` is er van alles mis.

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
