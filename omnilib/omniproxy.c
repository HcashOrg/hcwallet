#include <stdio.h>
#include <stdlib.h>
#include "omniproxy.h"

#if defined(_WIN64)||defined(_WIN32)

#include <windows.h>

typedef const char* (WINAPI *FunJsonCmdReq)(char *);
typedef int (WINAPI *FunOmniStart)(char *, char*);
typedef int (WINAPI *FunSetCallback)(unsigned int,void *);

FunOmniStart    funOmniStart = NULL; //
FunSetCallback  funSetCallback=NULL;
FunJsonCmdReq   funJsonCmdReq = NULL; //
#define INDEX_CALLBACK_GoJsonCmdReq 1

void CLoadLibAndInit()
{
	printf("in LoadDllStart\n");

    char szFilePath[MAX_PATH + 1]={0};
    GetModuleFileNameA(NULL, szFilePath, MAX_PATH);
    (strchr(szFilePath, '\\'))[1] = 0;
    strcpy(szFilePath, "omnicored.dll");

	HINSTANCE hDllInst = LoadLibrary(szFilePath);
    if(!hDllInst)
    {
        //FreeLibrary(hDllInst);
        return;
    }

    funOmniStart = (FunOmniStart)GetProcAddress(hDllInst,"OmniStart");
    funJsonCmdReq= (FunJsonCmdReq)GetProcAddress(hDllInst,"JsonCmdReq");
    funSetCallback= (FunSetCallback)GetProcAddress(hDllInst,"SetCallback");

    if(funSetCallback!=NULL)
        funSetCallback(INDEX_CALLBACK_GoJsonCmdReq,JsonCmdReqOmToHc);

    printf("funJsonCmdReq=%d",funJsonCmdReq);
    return;
}

int COmniStart(char *pcArgs, char *pcArgs1)
{
    if(funOmniStart==NULL)
        return -1;
    return funOmniStart(pcArgs, pcArgs1);
}

const char* CJsonCmdReq(char *pcReq)
{
    if(funJsonCmdReq==NULL)
        return NULL;
    const char* ret = funJsonCmdReq(pcReq);
    return ret;
};

int CSetCallback(int iIndex,void* pCallback)
{
    if(funSetCallback==NULL) return -1;
    if(pCallback==NULL) return -1;
    return funSetCallback(iIndex,pCallback);
};

#else //for linux etc
extern int OmniStart(char *pcArgs, char *pcArgs1);
extern const char* JsonCmdReq(char *pcReq);
void CLoadLibAndInit()
{
    return;
}
int COmniStart(char *pcArgs, char *pcArgs1)
{
    return OmniStart(pcArgs, pcArgs1);
}
const char* CJsonCmdReq(char *pcReq)
{
    return JsonCmdReq(pcReq);
};

//maybe used in future
int CSetCallback(int iIndex,void* pCallback)
{
    return 0;
};

#endif //end of defined(_WIN64)||defined(_WIN32)
