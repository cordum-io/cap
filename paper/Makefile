.PHONY: all clean

all: cap-paper.pdf

cap-paper.pdf: main.tex references.bib
	latexmk -pdf -interaction=nonstopmode -halt-on-error -jobname=cap-paper main.tex

clean:
	latexmk -C -jobname=cap-paper main.tex
	rm -f cap-paper.bbl cap-paper.bcf cap-paper.blg cap-paper.run.xml
