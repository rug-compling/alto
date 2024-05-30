# alto(1)

v0.3.0, 2024-04-17


## Naam

alto - **AL**pino **TO**olkit


## Samenvatting

**alto** (_optie_ | _actie_ | _bestand_) ...


## Beschrijving

**alto** is een toolkit voor het werken met _Alpino dependency
structures_, kortweg Alpino-bestanden. Je kunt losse bestanden
samenvoegen tot een corpus, je kunt een corpus omzetten naar een ander
soort corpus, je kunt zoeken, transformeren, visualiseren, etc.


## Opties

**-e** _expression_
: show macro-expansion, and exit

**-f**
: overwrite existing files

**-i**
: read input filenames from stdin

**-m** _filename_
: use this macrofile for xpath (or use environment variable **ALTO_MACROFILE**)

**-n**
: mark matching node

**-o** _filename_
: output

**-r**
: replace xml in existing dact file

**-v** _name_**=**_value_
: set global variable (can be used multiple times)

**-1**
: use XPath version 1 for searching in DACT files

**-2**
: use XPath2 and XSLT2 (slow)

**-2p**
: use XPath2 (slow)

**-2s**
: use XSLT2 (slow)

**-w**
: suppress warnings

## Acties

**ds:ud**
: insert Universal Dependencies

**ds:noud**
: remove Universal Dependencies

**ds:extra**
: add extra attributes: **is_np**, **is_vorfeld**, **is_nachfeld**

**ds:minimal**
: removes all but essential entities and attributes

**fp:**_expression_
: filter by XPath _expression_

**tq:**_xqueryfile_
: transform with XQuery _xqueryfile_

**ts:**_stylefile_
: transform with XSLT _stylefile_

**tt:**_template_
: transform with _template_

**Tq:**_xqueryfile_
: like **tq**, match data as input

**Ts:**_stylefile_
: like **ts**, match data as input

**ac:item**
: item count

**ac:line**
: line count

**ac:node**
: count of cat, pos, postag, rel

**ac:word**
: count of lemma, root, sense, word

**ac:nw**
: combination of **ac:node** and **ac:word**

**vt:**_type_
: save tree as image, _type_ is one of: **dot**, **svg**, **png**, **eps**, **pdf**

**vm:**_type_
: save subtree as image

**vu:**_type_
: save Universal Dependencies as image, _type_ is one of **svg**, **png**, **eps**, **pdf**

**vx:**_type_
: save Extended Universal Dependencies as image


## Gebruik

De argumenten voor **alto** bestaan uit opties, acties en
bestandsnamen. Je kunt deze door elkaar gebruiken. Acties worden
uitgevoerd in de volgorde waarin ze zijn gegeven. Bestanden worden
verwerkt in de volgorde waarin ze zijn gegeven.

De verwerking verloopt zo:

```
    ( input1, input2, ... ) -> actie1 -> actie2 -> ... -> output
```

Namen van opties beginnen met een minus. Namen van acties beginnen met
twee letters en een dubbele punt.


### Het maken van een corpus

**alto -o** _output.dact *.xml_
: Plaats alle XML-bestanden _*.xml_ in het DACT-bestand _output.dact_. Wanneer
  de optie **-o** ontbreekt gaat de uitvoer naar _stdout_.

**alto** _input.dact_ **-o** _output.data.dz_
: Zet DACT-bestand _input.dact_ om naar een compact corpus.

**alto** _input1.data.dz input2.data.dz_ **-o** _output.dact_
: Voeg meerdere compacte corpora samen in één DACT-bestand.

**find . -name '*.xml' | alto -i -o** _output.zip_
: Met optie **-i** lees je namen van invoerbestanden van _stdin_,
  één naam per regel.

**alto -o** _corpus_**.dact** _file_**.xml -r**
: Gewoonlijk worden geen bestanden overschreven. Het is wel mogelijk om
  een XML-bestand in een bestaand DACT-bestand te vervangen of toe te
  voegen. Hiervoor moet je de optie **-r** gebruiken.

Geldige namen voor invoer:

***.xml**
: los XML-bestand met één geparste zin

***.dact** (of ***.dbxml**)
: een DACT-bestand is een snel doorzoekbare collectie van XML-bestanden

***.data.dz** (of ***.index**)
: een compact corpus is een gecomprimeerde verzameling XML-bestanden

***.zip**
: een verzameling XML-bestanden in een ZIP-bestand

_naam van een map_
: een directory met daarin losse XML-bestanden en/of DACT-bestanden,
  compacte corpora, ZIP-bestanden, of subdirectory's

Je kunt ook één of meer xml-bestanden uit een corpusbestand (DACT,
compact, ZIP) selecteren
als invoer:

```
    input.dact::file1.xml::file2.xml::file3.xml
```

Geldige namen voor uitvoer:

***.dact** (of ***.dbxml**)
: als de uitvoer bestaat uit XML-bestanden kun je die opslaan in één
  DACT-bestand

***.data.dz** (of ***.index**)
: een compact corpus is bedoeld voor het opslaan van XML-bestanden van
  geparste zinnen, maar je kunt er ook andere bestanden in opslaan

***.zip**
: voor het opslaan van bestanden in één ZIP-bestand

***.txt**
: alle uitvoer wordt samengevoegd en opgeslagen in één doorlopend
  tekstbestand

_naam van een map_
: de verwerking van elk individueel XML-bestand wordt als los bestand
  opgeslagen in de directory


### Alpino-bestanden veranderen

**alto** _input.dact_ **-o** _output.dact_ **ds:ud**
: Voeg Universal Dependencies toe.

**alto** _input.dact_ **-o** _output.dact_ **ds:noud**
: Verwijder Universal Dependencies.

**alto** _input.dact_ **-o** _output.dact_ **ds:extra**
: Voeg extra attributen toe: **is_np**, **is_vorfeld**, **is_nachfeld**.

**alto** _input.dact_ **-o** _output.dact_ **ds:minimal**
: Verwijder entity's en attributen tot alleen dat overblijft wat door
  de minimale Alpino-plugin voor TrEd wordt gebruikt. Zie:
  https://www.let.rug.nl/vannoord/alp/Alpino/tred/


### Zoeken en filteren

**alto** _input.dact_ **-o** _output.dact_ **fp:**_'//node[@root="fiets"]'_
: Maak een subcorpus met alleen de XML-bestanden die een match hebben voor
  de XPATH-expressie _//node[@root="fiets"]_.

**alto** _input.dact_ **fp:**_'//node[@root="fiets"]'_ **tt:%f**
: Doorzoek een corpus en print de uitvoer op _stdout_. De
  transformatie **tt:%f** zorgt ervoor dat niet de inhoud van het XML-bestand geprint
  wordt, maar de naam van het XML-bestand.

**alto** _input.dact_ **fp:**_'//node[%my_macro%]'_ **tt:%f -m** _macrofile_
: Zoek met gebruik van een macro. De macro _my_macro_ is gedefinieerd in
  _macrofile_. Je kunt ook de environment variabele
  **ALTO_MACROFILE** gebruiken om naar het macrobestand te wijzen. De
  optie **-m** heeft voorrang.
: Voor de syntax van het macrobestand, zie:
  https://rug-compling.github.io/dact/manual/#macros

**alto -e** _'//node[%my_macro%]'_ **-m** _macrofile_
: Dit laat de XPath-expressie zien na substitie van macro's. Gebruik dit
  om te testen.

Je kunt de actie **fp:** meerdere keren gebruiken, eerst met een
simpele expressie om het zoeken te beperken tot een klein aantal
XML-bestanden in het corpus, daarna een tweede, mogelijk tijdrovende
expressie voor het eindresultaat.

Een aantal opties beïnvloeden het zoeken en filteren:

**-m** _filename_
: Lees definities van macro's uit bestand _filename_.

**-n**
: Plaats een speciale markering op de nodes die matchen. Deze markering
  kan in een later stadium gebruik worden voor een transformatie.
  Zo'n markering ziet er zo uit:
: **&lt;node**...**&gt;&lt;data name="match"/&gt;**...**&lt;/node&gt;**

**-1**
: Gewoonlijk wordt bij het zoeken in een DACT-bestand door het eerste
  filter gebruik gemaakt van XPATH versie 2. Dit is gewoonlijk het snelst,
  maar niet altijd correct. Met de optie **-1** zorg je ervoor dat
  eerst alle bestanden uit het DACT-bestand worden gelezen, en daarna
  gefilter met XPATH versie 1.

**-2p**
: Gewoonlijk wordt XPATH versie 1 gebruikt wanneer er niet rechtstreeks in
  een DACT-bestand wordt gezocht. Met deze optie zorg je ervoor dat altijd
  XPATH versie 2 gebruikt wordt. Dit is aanzienlijk trager dan zoeken met
  versie 1.

**-2**
: Dit combineert de opties **-2p** en **-2s** (zie beneden).


### Transformeren met een stylesheet

**alto** _input.xml_ **tq:**_style.xq_
: Transformeer de invoer (in dit geval een enkel XML-bestand) met XQuery dmv
  het script _style.xq_.

**alto** _input.xml_ **ts:**_style.xsl_
: Transformeer de invoer met XSLT dmv
  het stylesheet _style.xsl_.

**alto** _input.dact_ **fp:**_'//node[@root="fiets"]'_ **Tq:**_style.xq_
: Transformeer de gematchte subtree met XQuery dmv
  het script _style.xq_.

**alto** _input.dact_ **fp:**_'//node[@root="fiets"]'_ **Ts:**_style.xsl_
: Transformeer de gematchte subtree met XSLT dmv
  het stylesheet _style.xsl_.

Een aantal opties beïnvloeden de transformatie:

**-n**
: Zie boven, onder kopje **Zoeken en filteren**.

**-v** _name_**=**_value_
: Definieer de globale variabele _name_ met de waarde _value_. Je
  kunt deze optie meerdere keren gebruiken. De variabelen **filename**
  en **corpusname** worden automatisch gezet.

**-2s**
: Gebruik XSLT versie 2. Default is versie 1. Versie 2 is aanzienlijk
  trager.

**-2**
: Dit combineert de opties **-2s** en **-2p** (zie boven).


### Transformeren met een template

**alto** _input.dact_ **fp:**_'//node[node[@root="fiets"]]'_ **tt:**_'%f\\t%S\\n%M\\n'_
: Voor elke match voor de XPATH-expressie, print de bestandnaam, de zin
  met het matchende deel gemarkeerd, en daaronder de dependency structure
  van de match.

De volgende vlaggen kun je altijd gebruiken:

**\\t**
: Tab.

**\\n**
: Newline.

**%%**
: Het procent-teken.

**%c**
: De naam van het corpus.

**%f**
: De naam van het XML-bestand.

**%F**
: Als de invoer een DACT-bestand is, een compact corpus, of een
  ZIP-bestand, dan gelijk aan **%c::%f**, anders gelijk aan **%f**.

**%b**
: De inhoud van het XML-bestand.

**%I**
: De sentence-ID.

**%s**
: De zin.

**%o**
: Alle comments, gescheiden door **\\n\\t**.

**%d**
: De metadata.

**%u**
: De Universal Dependencies. Wanneer de input al UD bevatten worden die
  gebruikt, anders worden ze berekend. Het gebruik van alleen **tt:%u**
  is sneller dan de combinatie **ds:ud tt:%u**.

De volgende vlaggen kun je gebruiken na zoeken met XPATH. Wanneer er
meerdere machtes zijn in hetzelfde XML-bestand, dan worden de resultaten
apart getoond, behalve voor de vlag **%j**.

**%i**
: ID van de matchende node.

**%j**
: IDs van alle machtende nodes, gescheiden door een spatie.

**%S**
: De zin met de woorden onder de matchende node gekleurd.

**%m**
: De gematchte subtree als XML-fragment.

**%M**
: De gematche subtree als een dependency structure.

**%w**
: De woorden onder de matchende node.

**%l**
: De lemma's onder de matchende node.

**%p**
: De waardes van `pt` van de woorden onder de matchende node.

**%P**
: De waardes van `postag` van de woorden onder de matchende node.

Je kunt in een vlag een getal zetten om aan te geven hoe breed de uitvoer
moet zijn. Met een minus ervoor wordt de tekst links uitgelijnd, zonder
minus rechts. Een voorbeeld:

```
    tt:'%-14f %8I'
```


### Aggregeren

**alto** _corpus.dact_ **fp:**_'//node[@pt="vnw"]/@lemma'_ **ac:item**
: Met **ac:item** tel je varianten. Dit voorbeeld telt alle lemma's die
  een voornaamwoord zijn.

Bovenstaand voorbeeld telt elke match, en elke match bestaat uit
één regel. Bij de volgende voorbeelden gebruiken we een transformatie
met XQuery uit het bestand _mwu.xq_ met deze inhoud:

```
    for $x in //node[@cat='mwu']
    return fn:concat(fn:string-join($x//node[@word]/@word, ' '), '&#10;')
```

**alto** _corpus.dact_ **fp:**_'//node[@cat="mwu"]'_ **tq:**_mwu.xq_
: Dit geeft een lijst met alle multi-word units in het corpus. Sommige
  XML-bestanden bevatten meerdere multi-word units, en die worden onder
  elkaar weergegeven. In dit voorbeeld valt dat niet op.

**alto _corpus.dact** **fp:**\fI'//node[@cat="mwu"]'_ **tq:**_mwu.xq_ **ac:item**
: Wanneer je gaat tellen zul je zien dat sommige items uit meerdere
  regels bestaan, en ook die items worden als geheel geteld.

**alto _corpus.dact** **fp:**\fI'//node[@cat="mwu"]'_ **tq:**_mwu.xq_ **ac:line**
: Als je telt met **ac:line** dan wordt elk item gesplitst in regels, en de regels worden apart
  geteld. Dat is wat je in dit voorbeeld waarschijnlijk wilt.

**alto** _corpus.dact_ **fp:**_'//node[@root="fiets"]'_ **ac:node**
: Met **ac:node** tel je de volgende attributen van de matchende node:
  cat, pos, postag, rel

**alto** _corpus.dact_ **fp:**_'//node[@root="fiets"]'_ **ac:word**
: Met **ac:word** tel je de volgende attributen van de matchende node:
  lemma, root, sense, word

**alto** _corpus.dact_ **fp:**_'//node[@root="fiets"]'_ **ac:nw**
: **ac:nw** combineert **ac:node** en **ac:word**.


### visualiseren

**alto -n** _corpus.dact_ **fp:**_'//node[@root="fiets"]'_ **vt:png -o** _output_
: Met de actie **vt:png** maak je een PNG-afbeelding van de boom van de
  zin. In dit voorbeeld doe je dit alleen voor de zinnen die voldoen aan
  de XPATH-expressie, en de optie **-n** zorgt ervoor dat de matchende
  nodes in de boom een kleur krijgen. In dit voorbeeld worden alle
  PNG-afbeeldingen opgeslagen in de directory _output_. Behalve
  **png** kun je ook deze uitvoerformaten kiezen:
  **dot**, **svg**, **eps**, **pdf**.

**alto** _corpus.dact_ **fp:**_'//node[@root="fiets"]'_ **vm:png -o** _output_
: Met de actie **vm:png** doe je bijna hetzelfde als met **vt:png**,
  naar nu sla je alleen de subboom op van de matchende node. De optie
  **-n** heeft geen effect.

**alto** _corpus.dact_ **vu:png -o** _output_
: Met **vu:png** maak je een PNG-afbeelding van de Universal
  Dependencies. Bevat de invoer al UD, dan worden die gebruikt, anders
  worden ze alsnog afgeleid. Bestanden waarvoor het afleiden van UD
  mislukt worden overgeslagen. Behalve **png** kun je ook deze
  uitvoerformaten kiezen: **svg**, **eps**, **pdf**.

**alto** _corpus.dact_ **vx:png -o** _output_
: Met **vx:png** maak je PNG-afbeeldingen van de Extended Universal
  Dependencies. Verder is dit voorbeeld gelijk aan het vorige.


## Environment

**ALTO_MACROFILE**
: Bevat de naam van het bestand met macrodefinities. Genegeerd als de
  optie **-m** gebruikt wordt.
: Voor het gebruik van macro's, zie:
  https://rug-compling.github.io/dact/manual/#macros

**TEMP**
: Naam van directory waar **alto** tijdelijke bestanden opslaat.

**TMP**
: Wordt gebruikt in plaats van **TEMP** als die variabele leeg is.


## Auteur

Peter Kleiweg


## Bugs

https://github.com/rug-compling/alto/issues

