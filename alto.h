#ifndef ALTO_H
#define ALTO_H

#ifdef __cplusplus
extern "C"
{
#endif

    typedef struct c_xqilla_t *c_xqilla;
    typedef struct c_xqilla_result_t *c_xqilla_result;

    typedef enum
    {
        langXPATH,
        langXQUERY,
        langXSLT
    } Language;

    c_xqilla xq_setup(char const *query, Language language, int nvars, char const **vars);
    c_xqilla_result xq_call(c_xqilla xq, char const *xmlfile, char const *suffix, int nvars, char const **vars);
    int xq_error(c_xqilla_result xq);
    int xq_setup_error(c_xqilla xq);
    char const *xq_text(c_xqilla_result xq);
    void xq_free(c_xqilla_result xq);

#ifdef __cplusplus
}
#endif

#endif /* ALTO */
