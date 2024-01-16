#include "alto.h"
// #include <xercesc/framework/MemBufInputSource.hpp>
#include <string>
#include <xercesc/framework/MemoryManager.hpp>
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

            // AutoDelete<DynamicContext> context(xqilla.createContext(lang));
            // AutoDelete<XQQuery> qq(xqilla.parse(X(query), context));
            DynamicContext *context = xqilla.createContext(lang);
            XQQuery *qq = xqilla.parse(X(query), context);

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

        /*
            if (qq != NULL)
            {
                delete qq;
            }
            if (context != NULL)
            {
                delete context;
            }
        */

        return xqr;
    }
}
