#!/usr/bin/env python3

import sys
import os
import re
import textwrap

def printline(formatstr, shortOpt, longOpt, argType, envVar, descr):
    #envVar = envVar.replace("_", "\_")
    descrLines = textwrap.wrap(descr, width=60)
    opts = longOpt
    if shortOpt != "":
        opts = opts + " " + shortOpt
    if len(descrLines) == 0:
        descrLines = ["-"]
    for descrLine in descrLines:
        print(formatstr %  (opts, argType, envVar, descrLine))
        argType = ""
        envVar = ""
        opts = ""
        
if len(sys.argv) < 2:
    print("usage: cmd [ go_file.go ... ]")
    exit(1)

cmd=sys.argv[1]

print("running command: ", cmd, file=sys.stderr)
output = os.popen(cmd).read()
print("output: ", output, file=sys.stderr)

state = "start"

arglist = []
for line in output.splitlines():
    print (state, line, file=sys.stderr)    
    if line.strip() == "":
        if state!="nop":
            print("")
            state = "nop"
        continue
    if line.startswith("Available Commands:"):
        state = "available"
        print("**"+line+"**\n")
        continue
    if line.startswith("Usage:"):
        state = "usage"
        print("**"+line+"**\n")
        continue
    if line.startswith("Flags:"):
        state = "flg"
        print("**"+line+"**\n")
        continue
    if state == "usage" or state=="available":
        print("* %s" % line.strip())
    if state == "start":
        print("%s\n\n" % line)
        state = "nop"
    if state == "flg":
        shortOpt = line[0:6].strip()
        spec = line[6:51].strip()
        descr = line[51:].strip()
        (longOpt, sep, optType) = spec.partition(" ")
        longOpt = longOpt.strip()
        optType = optType.strip()
        print("short=", shortOpt, "long=", longOpt, "type=", optType, "descr=", descr, file=sys.stderr)
        arglist.append((shortOpt, longOpt, optType, descr))

varmap = {}
typesmap = {}
for gofile in sys.argv[2:]:
    print ("processing file ", gofile, file=sys.stderr)
    with open(gofile) as fd:
        output = fd.read()
        for line in output.splitlines():
            line = line.strip()
            #if re.match("^[_a-z][a-zA-Z0-9_]*\\.Flags\\(\\)\\.[A-Z][A-Za-z0-9]*Var\\(.*$", line):
            if re.match("^[_a-z][a-zA-Z0-9_]*\\.Flags\\(\\)\\.[A-Z][A-Za-z0-9_]*Var.*$", line):
                print("MATCHED", line,  file=sys.stderr)
                argMatch = re.search("\"([a-z][a-z0-9-]+)\"", line)
                if not argMatch:
                    continue
                argName = argMatch.group(1)
                print("ARG", argName,  file=sys.stderr)
                argMatch = re.search("env.(Strings?|ParseNum|ParseBool|ParseDuration|ParseInt64|ParseFloat64|ParseFloat)FromEnv\\(\"([A-Z0-9_]+)\"", line)
                if not argMatch:
                    continue
                varName = argMatch.group(2)
                print("VAR", varName,  file=sys.stderr)
                varType = argMatch.group(1)
                if varType == "ParseBool":
                    typesmap[argName]="bool"
                varmap[argName] = varName
                
                
formatstr = f"| %-39s | %-11s | %-44s | %-60s |"

printline(formatstr, "", "Argument",  "Type", "Environment Variable", "Description")
printline(formatstr, "", "---------------------------------------", "-----------", "--------------------------------------------", "------------------------------------------------------------")

for arg in arglist:
    descr = arg[3].replace("|",",")
    envVar = ""
    argName = arg[1].removeprefix("--")
    if argName in varmap:
        envVar = varmap[argName]
    typeName = arg[2]
    if argName in typesmap:
        typeName = typesmap[argName]
    
    printline(formatstr, arg[0], arg[1], typeName, envVar, descr)
    # print("|", arg[0], arg[1], " | ", rg[2], " | ", envVar, " | ", descr, " |")


