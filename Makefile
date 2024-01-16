
%_v6 : %.go template.go alto.cc alto.h
	envgo -d6 go build -o $@ .

% : %.go template.go alto.cc alto.h
	envgo -d2 go build .

all: \
	alto \
	alto_v6
