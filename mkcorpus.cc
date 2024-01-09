#include "mkcorpus.h"
// #include <xercesc/framework/MemBufInputSource.hpp>
#include <string>
#include <xercesc/framework/MemoryManager.hpp>
#include <xqilla/exceptions/XQException.hpp>
#include <xqilla/xqilla-simple.hpp>

extern "C"
{
    XQilla xqilla;

    struct c_xqilla_t
    {
        DynamicContext *context;
        XQQuery *query;
        std::string text;
        bool error;
    };

    c_xqilla prepare(char const *stylesheet, Language language)
    {

        c_xqilla xq;
        xq = new c_xqilla_t;
        xq->error = false;

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
            xq->query = xqilla.parse(X(stylesheet), xq->context);
        }
        catch (XQException &xe)
        {
            xe.printDebug(X("prepare(stylesheet, language)"));
            xq->error = true;
        }

        return xq;
    }

    void setname(c_xqilla xq, char const *name, char const *value)
    {
        Item::Ptr val = xq->context->getItemFactory()->createUntypedAtomic(X(value), xq->context);
        xq->context->setExternalVariable(X(name), val);
    }

    char const *run(c_xqilla xq, char const *xmlfile, char const *suffix)
    {
        xq->error = false;

        /*
        xercesc::MemBufInputSource source((const unsigned char *)xmldata,
                                          strlen(xmldata), UTF8("TEST"));
        xercesc::BinInputStream stream = source.makeStream ()
        */

        try
        {
            // Node::Ptr item = context->parseDocument(source);
            // Sequence seq = Sequence(item);
            Sequence seq = xq->context->resolveDocument(X(xmlfile));
            if (!seq.isEmpty() && seq.first()->isNode())
            {
                xq->context->setContextItem(seq.first());
                xq->context->setContextPosition(1);
                xq->context->setContextSize(1);
            }
        }
        catch (XQException &xe)
        {
            xe.printDebug(X("run(xmlfile)"));
            xq->error = true;
            return "";
        }

        Result result = xq->query->execute(xq->context);

        xq->text = "";

        Item::Ptr item;
        while ((item = result->next(xq->context)) != NULL)
        {
            xq->text += UTF8(item->asString(xq->context));
            xq->text += suffix;
        }

        return xq->text.c_str();
    }

    int match(c_xqilla xq, char const *xmlfile)
    {
        xq->error = false;
        int n = 0;

        /*
        xercesc::MemBufInputSource source((const unsigned char *)xmldata,
                                          strlen(xmldata), UTF8("TEST"));
        xercesc::BinInputStream stream = source.makeStream ()
        */

        try
        {
            // Node::Ptr item = context->parseDocument(source);
            // Sequence seq = Sequence(item);
            Sequence seq = xq->context->resolveDocument(X(xmlfile));
            if (!seq.isEmpty() && seq.first()->isNode())
            {
                xq->context->setContextItem(seq.first());
                xq->context->setContextPosition(1);
                xq->context->setContextSize(1);
            }
        }
        catch (XQException &xe)
        {
            xe.printDebug(X("match(xmlfile)"));
            xq->error = true;
            return n;
        }

        Result result = xq->query->execute(xq->context);

        Item::Ptr item;
        while ((item = result->next(xq->context)) != NULL)
            n++;

        return n;
    }

    int xq_error(c_xqilla xq)
    {
        return xq->error ? 1 : 0;
    }
}
