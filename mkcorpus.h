#ifndef MKCORPUS_H
#define MKCORPUS_H

#ifdef __cplusplus
extern "C"
{
#endif

    typedef struct c_xqilla_t *c_xqilla;

    typedef enum
    {
        langXPATH,
        langXQUERY,
        langXSLT
    } Language;

    c_xqilla prepare(char const *stylesheet, Language language);
    void setname(c_xqilla xq, char const *name, char const *value);
    char const *run(c_xqilla xq, char const *xmlfile, char const *suffix);
    int match(c_xqilla xq, char const *xmlfile);
    int xq_error(c_xqilla xq);

#ifdef __cplusplus
}
#endif

#endif /* MKCORPUS */
