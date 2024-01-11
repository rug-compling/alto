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
