#ifndef MKCORPUS_H
#define MKCORPUS_H

#ifdef __cplusplus
extern "C"
{
#endif

    typedef struct c_xqilla_result_t *c_xqilla_result;

    typedef enum
    {
        langXPATH,
        langXQUERY,
        langXSLT
    } Language;

    c_xqilla_result xq_call(char const *xmlfile, char const *query, Language language, char const *suffix, int nvars,
                            char const **vars);
    int xq_error(c_xqilla_result xq);
    char const *xq_text(c_xqilla_result xq);
    void xq_free(c_xqilla_result xq);

#ifdef __cplusplus
}
#endif

#endif /* MKCORPUS */
