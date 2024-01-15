
%_v6 : %.go template.go mkcorpus.cc mkcorpus.h
	envgo -d6 go build -o $@ .

% : %.go template.go mkcorpus.cc mkcorpus.h
	envgo -d2 go build .

all: \
	mkcorpus \
	mkcorpus_v6
