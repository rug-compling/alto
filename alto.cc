#include "alto.h"
#include <string>
#include <xqilla/exceptions/XQException.hpp>
#include <xqilla/xqilla-simple.hpp>

extern "C"
{
    XQilla xqilla;

    struct c_xqilla_result_t
    {
        std::string text;
        int error;
    };

    struct c_xqilla_t
    {
        DynamicContext *context;
        XQQuery *qq;
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

    int xq_setup_error(c_xqilla xq)
    {
        return xq->error;
    }

    char const *xq_text(c_xqilla_result xq)
    {
        return xq->text.c_str();
    }

    c_xqilla xq_setup(char const *query, Language language, int nvars, char const **vars)
    {
        c_xqilla xq = new c_xqilla_t;
        xq->error = 0;

        XQilla::Language lang;
        switch (language)
        {
        case langXPATH:
            lang = XQilla::XPATH2;
            break;
        case langXQUERY:
            lang = XQilla::XQUERY;
            break;
        case langXSLT:
            lang = XQilla::XSLT2;
            break;
        }

        try
        {

            xq->context = xqilla.createContext(lang);
            xq->qq = xqilla.parse(X(query), xq->context, 0); // , XQilla::NO_ADOPT_CONTEXT);

            for (int i = 0; i < nvars; i++)
            {
                Item::Ptr val = xq->context->getItemFactory()->createUntypedAtomic(X(vars[2 * i + 1]), xq->context);
                xq->context->setExternalVariable(X(vars[2 * i]), val);
            }
        }
        catch (XQException &xe)
        {
            xe.printDebug(X("xq_setup"));
            xq->error = 1;
        }

        return xq;
    }

    c_xqilla_result xq_call(c_xqilla xq, char const *xmlfile, char const *suffix, int nvars, char const **vars)
    {
        c_xqilla_result xqr = new c_xqilla_result_t;
        xqr->text = "";
        xqr->error = 0;

        try
        {

            for (int i = 0; i < nvars; i++)
            {
                Item::Ptr val = xq->context->getItemFactory()->createUntypedAtomic(X(vars[2 * i + 1]), xq->context);
                xq->context->setExternalVariable(X(vars[2 * i]), val);
            }

            Sequence seq = xq->context->resolveDocument(X(xmlfile));
            if (!seq.isEmpty() && seq.first()->isNode())
            {
                xq->context->setContextItem(seq.first());
                xq->context->setContextPosition(1);
                xq->context->setContextSize(1);
            }

            Result result = xq->qq->execute(xq->context);

            Item::Ptr item;
            while ((item = result->next(xq->context)) != NULL)
            {
                xqr->text += UTF8(item->asString(xq->context));
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
