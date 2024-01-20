#include "alto.h"
#include <string>
#include <xqilla/exceptions/XQException.hpp>
#include <xqilla/xqilla-simple.hpp>

#include <db.h>
#ifdef DB_VERSION_FAMILY
#define XPATH XQilla::XPATH3
#define XQUERY XQilla::XQUERY3
#define XSLT XQilla::XSLT3
#else
#define XPATH XQilla::XPATH2
#define XQUERY XQilla::XQUERY
#define XSLT XQilla::XSLT2
#endif

extern "C"
{
    XQilla xqilla;

    struct c_xqilla_result_t
    {
        std::string text;
        int error;
    };

    void xq_free(c_xqilla_result xq)
    {
        delete xq;
    }

    int xq_error(c_xqilla_result xq)
    {
        return xq->error;
    }

    char const *xq_text(c_xqilla_result xq)
    {
        return xq->text.c_str();
    }

    c_xqilla_result xq_call(char const *xmlfile, char const *query, Language language, char const *suffix, int nvars,
                            char const **vars)
    {
        c_xqilla_result xqr = new c_xqilla_result_t;
        xqr->text = "";
        xqr->error = 0;

        XQilla::Language lang;
        switch (language)
        {
        case langXPATH:
            lang = XPATH;
            break;
        case langXQUERY:
            lang = XQUERY;
            break;
        case langXSLT:
            lang = XSLT;
            break;
        }

        try
        {

            AutoDelete<DynamicContext> context(xqilla.createContext(lang));
            AutoDelete<XQQuery> qq(xqilla.parse(X(query), context, 0, XQilla::NO_ADOPT_CONTEXT));

            for (int i = 0; i < nvars; i++)
            {
                Item::Ptr val = context->getItemFactory()->createUntypedAtomic(X(vars[2 * i + 1]), context);
                context->setExternalVariable(X(vars[2 * i]), val);
            }

            Sequence seq = context->resolveDocument(X(xmlfile));
            if (!seq.isEmpty() && seq.first()->isNode())
            {
                context->setContextItem(seq.first());
                context->setContextPosition(1);
                context->setContextSize(1);
            }

            Result result = qq->execute(context);

            Item::Ptr item;
            while ((item = result->next(context)) != NULL)
            {
                xqr->text += UTF8(item->asString(context));
                xqr->text += suffix;
            }
        }
        catch (XQException &xe)
        {
            xe.printDebug(X("xq_call"));
            xqr->error = 1;
        }

        return xqr;
    }
}
