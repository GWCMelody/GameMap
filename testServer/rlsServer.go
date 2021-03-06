//stuff to fix: don't create a new game when client tries to join one that doesn't exist
package main

import (
    "fmt"
    "net"
    "os"
    "bytes"
    "strings"
    "encoding/json"
    "io/ioutil"
    "strconv"
    "time"
    "sync"
    "math/rand"
    "math"
)

//general constants
const (
    HostName = "10.0.1.128"
    //HostName = "127.0.0.1"
    Port = "8889" //8888 is for stable server. 8889 is for test server. in the future we may need different ports for each person
    ConnType = "udp"
    LocPlaces int = 5
    PacketLossChance = 0
)

//networking protocol codes
const (
	UUID uint8 = 0
	UUID_CK uint8 = 1
	HEART uint8 = 2
	BEAT uint8 = 3
	CONN uint8 = 4
	TOGGLE_RDL uint8 = 5
	TOGGLE_PP uint8 = 6
	SIM_CLIENT uint8 = 7
	CHECK_ID uint8 = 8
	CHECK_ID_H uint8 = 9
	CHECK_ID_J uint8 = 10
	CHECK_NAME uint8 = 11
	BK uint8 = 12
	BK_CK uint8 = 13
	TEAM uint8 = 14
	LOC uint8 = 15
	WARD uint8 = 16
	DEAD uint8 = 17
	DC uint8 = 18
	RESET uint8 = 19
	BP uint8 = 20
	RP uint8 = 21
	BP_CK uint8 = 22
	RP_CK uint8 = 23
	BP_CT uint8 = 24
	RP_CT uint8 = 25
	REC_BP uint8 = 26
	REC_RP uint8 = 27
	WARD_CK uint8 = 28
	TEAM_CK uint8 = 29
	DEAD_CK uint8 = 30
	DC_CK uint8 = 31
)

//dictionary of conn objects to client objects
var addrToChannel map[string]chan string
var addrToClient map[string]*Client
//the master json object
var master *Master
//this is explained below
var posInc map[uint8]int
var mutex sync.Mutex //lock this with actions regarding connToClient or printPeriodicals
var printPeriodicals bool
var packetConn net.PacketConn
var debug = true

func main() {
    /*initializes the addrToClient array. the design is kinda weird, i basically treat
    rlsServer itself as its own class with its own getters and setters. this way, i
    don't forget use mutex before accessing or changing any data since locking the
    mutex is built in (by the code, not by golang) to all getters and setters*/
    setAddrToClient(make(map[string]*Client))
    setAddrToChannel(make(map[string]chan string))

    //sets print periodicals to true. you might prefer to set this to false by default
    setPrintPeriodicals(false)

    //position increment. the amount of cells to increment in the string array after every command
    posInc = map[uint8]int {
        //explanations of commands are in the simulated client guide doc
        UUID: 2,
        HEART: 1,
        CONN: 1,
        TOGGLE_RDL: 1,
        TOGGLE_PP: 1,
        SIM_CLIENT: 1,
        CHECK_ID_H: 2,
        CHECK_ID_J: 2,
        CHECK_NAME: 2,
        BK: 2,
        TEAM: 2,
        LOC: 3,
        WARD: 3,
        DEAD: 2,
        DC: 1,
        RESET: 1,
        BP: 4,
        RP: 4,
        BP_CT: 1,
        RP_CT: 1,
        REC_BP: 2,
        REC_RP: 2,
        WARD_CK: 4,
        TEAM_CK: 3,
        DEAD_CK: 3,
        DC_CK: 2,
    }

    //sets the default state of the master json
    setMaster(baseMaster())

    //broadcast is a single forever loop that takes care of all the writes
    go broadcast()

    //send data through a channel based on the data's address for another goroutine to process the data

    //creates the listener (for connections, not for data)
    //err has to be initialized instead of using := notation because packetConn is a globalvar
    var err error
    packetConn, err = net.ListenPacket(ConnType, HostName + ":" + Port)

    //crashes if there's an error in creating the listener (for example, if another instance of the server is running)
    if err != nil {
        fmt.Printf("Error listening: %v\n", err.Error())
        os.Exit(1)
    } else {
        fmt.Printf("Listening on %s:%s\n", HostName, Port)
    }

    defer packetConn.Close()

    //the forever loop that reads
    for {
        //read for data
        /*create the buffer here instead of just once outside the loop because
            otherwise it doesn't clear*/
        buf := make([]byte, 2048)
        _, addrObject, err := packetConn.ReadFrom(buf)
        addr := addrObject.String()
        bufString := string(bytes.Trim(buf, "\x00"))
        _, ok := getClient(addr)

        if (!ok) {
            channel := make(chan string)
            setChannel(addr, channel)

            //if this is a new address, create a new routine that listens for data from that connection
            go processData(addr)

            //send the data to the new goroutine
            getChannel(addr) <- bufString
        } else {
            getChannel(addr) <- bufString
        }

        if err != nil {
            fmt.Printf("Error reading: %v\n", err.Error())
            //think about not exiting for a failed connection
            os.Exit(1)
        }
    }

    return
}

func processData(addr string) {
    rdlEnabled := true
    channel := getChannel(addr)

    for {
        content := <-channel
		info := strings.Split(content, ":")
        writeString := ""
        printString := ""
        posInSlice := 0
        pp := getPrintPeriodicals()

        printReadClient, printReadOk := getClient(addr)

        if (printReadOk) {
            printRead(printReadClient, info, pp, addr)
        }

        for ;posInSlice < len(info) - 1; {
            validBuffer := true

            bufType, err := convertBufType(info[posInSlice])

            if (err != nil) {
                fmt.Printf("buffer type convert to int error: %v\n", err)
                posInSlice++
                continue
            }

            client, clientOk := getClient(addr)

            //skips everything except uuid, togglePP, toggleRDL, and connected if client is nil
            if (!clientOk && (bufType != UUID && bufType != TOGGLE_PP && bufType != TOGGLE_RDL && bufType != CONN)) {
                fmt.Printf("client doesn't exist and non-non-client specific command sent\n")
                posInSlice++
                continue
            }

            //protects against valid types without enough data following them
            if (posInSlice + posInc[bufType] >= len(info)) {
                //it's ++ instead of break in the case that loc:toggleRDL: or smth is sent
                //theoretically this can never happen
                fmt.Printf("Valid type sent without enough data\n")
                posInSlice++
                continue
            }

            switch bufType {
            case UUID:
                uuid := info[posInSlice + 1]
                foundUUID := false

                for _, thisClient := range getAddrToClient() {
                    if (thisClient.getUUID() == uuid) {
                        setClient(addr, thisClient)
                        foundUUID = true
                    }
                }

                if (!foundUUID) {
                    client = &Client{}
                    client.setUUID(uuid)
                    setClient(addr, client)
                }

                additionalString := fmt.Sprintf("%d:%s:", UUID_CK, uuid)
                writeString += additionalString
                printString += additionalString
            case HEART:
                additionalString := fmt.Sprintf("%d:", BEAT)
                writeString += additionalString

                if (pp) {
                    printString += additionalString
                }
            case CONN: //why is connected used
            case TOGGLE_RDL:
                rdlEnabled = !rdlEnabled

                if (!rdlEnabled) {
                    //conn.SetReadDeadline(time.Time{})
                }
            case TOGGLE_PP:
                setPrintPeriodicals(!getPrintPeriodicals())
            case SIM_CLIENT:
                client.setSimClient(true)
            case CHECK_ID_H: //h stands for host
                readGameID := info[posInSlice + 1]

                if (readGameID != "") {
                    idTaken := getMaster().checkIDTaken(readGameID)

                    if (!idTaken) {
                        thisGame := &Game{}
                        thisGame.constructor()
                        thisGame.setGameID(readGameID)
                        getMaster().addGame(thisGame)
                        client.setGame(thisGame)
                    } else {
                        client.setGame(getMaster().getGame(readGameID))
                    }

                    additionalString := ""

                    if (client.getGame().getGameID() == readGameID) { //special case of client not receiving checkID:false: the first time
                        additionalString = fmt.Sprintf("%d:false:", CHECK_ID)
                    } else {
                        additionalString = fmt.Sprintf("%d:%t:", CHECK_ID, idTaken)
                    }

                    writeString += additionalString
                    printString += additionalString
                }
            case CHECK_ID_J: //j stands for join
                readGameID := info[posInSlice + 1]

                if (readGameID != "") {
                    idTaken := getMaster().checkIDTaken(readGameID)

                    if (idTaken) {
                        client.setGame(getMaster().getGame(readGameID))
                    }

                    additionalString := ""

                    if (client.getGame() != nil && client.getGame().getGameID() == readGameID) { //special case of client not receiving checkID:false: the first time
                        additionalString = fmt.Sprintf("%d:true:", CHECK_ID)
                    } else {
                        additionalString = fmt.Sprintf("%d:%t:", CHECK_ID, idTaken)
                    }

                    writeString += additionalString
                    printString += additionalString
                }
            case CHECK_NAME:
                var readName = info[posInSlice + 1]

                if (readName != "") {
                    if (client.getGame() != nil) {
                        var nameTaken bool

                        if (client.getPlayer() != nil && client.getPlayer().getName() == readName) {
                            nameTaken = false
                        } else {
                            nameTaken = client.getGame().checkNameTaken(readName)

                            if (!nameTaken) {
                                thisPlayer := &(Player{})
                                thisPlayer.setName(readName)
                                client.setPlayer(thisPlayer)
                                thisGame := client.getGame()
                                thisGame.addPlayer(thisPlayer)
                                thisPlayer.constructor(thisGame.getPlayerNames())

                                myName := thisPlayer.getName()

                                for _, name := range thisGame.getPlayerNames() {
                                    thisGame.getPlayer(name).setSendTo("ward", myName, true)
                                    thisGame.getPlayer(name).setSendTo("team", myName, true)
                                    thisGame.getPlayer(name).setSendTo("dead", myName, true)
                                }
                            }
                        }

                        additionalString := fmt.Sprintf("%d:%t:", CHECK_NAME, nameTaken)
                        writeString += additionalString
                        printString += additionalString
                    } else {
                        fmt.Printf("A client without a game is trying to add a player\n")
                    }
                }
            case BK:
                bk, err := strconv.ParseBool(info[posInSlice + 1])

                if (err == nil) {
                    if (client.getPlayer() != nil && client.getGame() != nil) {
                        client.getPlayer().setBK(bk)

                        additionalString := fmt.Sprintf("%d:%t:", BK_CK, bk)
                        writeString += additionalString
                        printString += additionalString
                    } else {
                        fmt.Printf("A client without a game/player is trying to set bk\n")
                    }
                } else {
                    fmt.Printf("Error parsing bk: %v\n", err)
                    validBuffer = false
                }
            case TEAM:
                team := info[posInSlice + 1]

                if (client.getPlayer() != nil && client.getGame() != nil) {
                    thisPlayer := client.getPlayer()

                    if (team != thisPlayer.getTeam()) {
                        thisPlayer.setWardLoc(200, 200)
                        thisPlayer.makeSendTrue("ward", client.getGame().getPlayerNames())
                    }

                    thisPlayer.setTeam(team)
                    thisPlayer.makeSendTrue("team", client.getGame().getPlayerNames())
                    additionalString := fmt.Sprintf("%d:%s:", TEAM_CK, team)
                    writeString += additionalString
                    printString += additionalString
                } else {
                    fmt.Printf("A client without a game/player is trying to set team\n")
                }
            case LOC:
                lat, err1 := strconv.ParseFloat(info[posInSlice + 1], 64)
                long, err2 := strconv.ParseFloat(info[posInSlice + 2], 64)

                if (err1 == nil && err2 == nil) {
                    if (client.getPlayer() != nil && client.getGame() != nil) {
                        client.getPlayer().setLoc(lat, long)
                    } else {
                        fmt.Printf("A client without a game/player is trying to set loc\n")
                    }
                } else {
                    fmt.Printf("Error parsing location. Lat error: %v; long error: %v\n", err1, err2)
                    //setting validBuffer on a failed parse makes sure any commands after loc are kept
                    //for example, if loc:ward:1:1: is sent
                    validBuffer = false
                }
            case WARD:
                lat, err1 := strconv.ParseFloat(info[posInSlice + 1], 64)
                long, err2 := strconv.ParseFloat(info[posInSlice + 2], 64)

                if (err1 == nil && err2 == nil) {
                    if (client.getPlayer() != nil && client.getGame() != nil) {
                        client.getPlayer().setWardLoc(lat, long)
                        client.getPlayer().makeSendTrue("ward", client.getGame().getPlayerNames())
                        additionalString := fmt.Sprintf("%d:%f:%f:", WARD_CK, lat, long)
                        writeString += additionalString
                        printString += additionalString
                    } else {
                        fmt.Printf("A client without a game/player is trying to set ward loc\n")
                    }
                } else {
                    fmt.Printf("Error parsing ward location. Lat error: %v; long error: %v\n", err1, err2)
                    validBuffer = false
                }
            case DEAD:
                dead, err := strconv.ParseBool(info[posInSlice + 1])

                if (err == nil) {
                    if (client.getPlayer() != nil && client.getGame() != nil) {
                        client.getPlayer().setDead(dead)
                        client.getPlayer().makeSendTrue("dead", client.getGame().getPlayerNames())
                        additionalString := fmt.Sprintf("%d:%t:", DEAD_CK, dead)
                        writeString += additionalString
                        printString += additionalString
                    } else {
                        fmt.Printf("A client without a game/player is trying to set dead\n")
                    }
                } else {
                    fmt.Printf("Error parsing dead: %v\n", err)
                    validBuffer = false
                }
            case DC:
                client.playerDisconnectActions()
                additionalString := fmt.Sprintf("%d:", DC_CK)
                writeString += additionalString
                printString += additionalString
            case RESET:
                client.getGame().resetSettings()
            case BP:
                index, err1 := strconv.ParseInt(info[posInSlice + 1], 10, 64)
                lat, err2 := strconv.ParseFloat(info[posInSlice + 2], 64)
                long, err3 := strconv.ParseFloat(info[posInSlice + 3], 64)

                if (err1 == nil && err2 == nil && err3 == nil) {
                    if (client.getGame() != nil) {
                        client.getGame().addBorderPoint(int(index), lat, long)
                        additionalString := fmt.Sprintf("%d:%d:%f:%f:", BP_CK, index, lat, long)
                        writeString += additionalString
                        printString += additionalString
                    } else {
                        fmt.Printf("A client without a game is trying to add a border point\n")
                    }
                } else {
                    fmt.Printf("Error parsing border point location. Index error: %v; lat error: %v; long error: %v\n", err1, err2, err3)
                    /*setting validBuffer on a failed parse makes sure any commands after loc are kept
                    for example, if loc:ward:1:1: is sent*/
                    validBuffer = false
                }
            case RP:
                index, err1 := strconv.ParseInt(info[posInSlice + 1], 10, 64)
                lat, err2 := strconv.ParseFloat(info[posInSlice + 2], 64)
                long, err3 := strconv.ParseFloat(info[posInSlice + 3], 64)

                if (err1 == nil && err2 == nil && err3 == nil) {
                    if (client.getGame() != nil) {
                        client.getGame().addRespawnPoint(int(index), lat, long)
                        additionalString := fmt.Sprintf("%d:%d:%f:%f:", RP_CK, index, lat, long)
                        writeString += additionalString
                        printString += additionalString
                    } else {
                        fmt.Printf("A client without a game is trying to add a respawn point\n")
                    }
                } else {
                    fmt.Printf("Error parsing respawn point location. Index error: %v; lat error: %v; long error: %v\n", err1, err2, err3)
                    /*setting validBuffer on a failed parse makes sure any commands after loc are kept
                    for example, if loc:ward:1:1: is sent*/
                    validBuffer = false
                }
            case BP_CT:
                /*the design is that the client asks for the server to send game element packets instead of
                the server continously sending packets until the client asks it to stop. this way, the more
                expensive game element packet is sent after the less expensive request packet. it's like
                why you put less expensive checks first in an if statement.*/
                if (client.getPlayer() != nil && client.getGame() != nil) {
                    additionalString := fmt.Sprintf("%d:%d:", BP_CT, len(client.getGame().getBorderPoints()))
                    writeString += additionalString
                    printString += additionalString
                } else {
                    fmt.Printf("A client without a game/player is trying to set bp count\n")
                }
            case RP_CT:
                if (client.getPlayer() != nil && client.getGame() != nil) {
                    additionalString := fmt.Sprintf("%d:%d:", RP_CT, len(client.getGame().getRespawnPoints()))
                    writeString += additionalString
                    printString += additionalString
                } else {
                    fmt.Printf("A client without a game/player is trying to set rp count\n")
                }
            case REC_BP:
                index, err := strconv.ParseInt(info[posInSlice + 1], 10, 64)

                if (err == nil) {
                    if (client.getPlayer() != nil && client.getGame() != nil) {
                        client.setReceivingBP(int(index), true)
                    } else {
                        fmt.Printf("A client without a game/player is trying to set recBP\n")
                    }
                } else {
                    fmt.Printf("Error parsing index for recBP: %v\n", err)
                }
            case REC_RP:
                index, err := strconv.ParseInt(info[posInSlice + 1], 10, 64)

                if (err == nil) {
                    if (client.getPlayer() != nil && client.getGame() != nil) {
                        client.setReceivingRP(int(index), true)
                    } else {
                        fmt.Printf("A client without a game/player is trying to set recRP\n")
                    }
                } else {
                    fmt.Printf("Error parsing index for recRP: %v\n", err)
                }
            case WARD_CK: //ward check
                name := info[posInSlice + 1]
                wardLat, err1 := strconv.ParseFloat(info[posInSlice + 2], 64)
                wardLong, err2 := strconv.ParseFloat(info[posInSlice + 3], 64)
                myName := client.getPlayer().getName()
                myGame := client.getGame()

                if (err1 == nil && err2 == nil) {
                    if (client.getPlayer() != nil && client.getGame() != nil) {
                        thisPlayer := myGame.getPlayer(name)

                        if (thisPlayer != nil) {
                            if (thisPlayer.getWardLat() == wardLat && thisPlayer.getWardLong() == wardLong) {
                                thisPlayer.setSendTo("ward", myName, false)
                            } else {
                                thisPlayer.setSendTo("ward", myName, true)
                            }
                        } else {
                            fmt.Printf("Name %s sent in wardCk doesn't exist\n", name)
                        }
                    } else {
                        fmt.Printf("A client without a game/player is trying to check ward\n")
                    }
                } else {
                    fmt.Printf("Error parsing wardCk location. Lat error: %v; long error: %v\n", err1, err2)
                    validBuffer = false
                }
            case TEAM_CK:
                name := info[posInSlice + 1]
                team := info[posInSlice + 2]
                myName := client.getPlayer().getName()
                myGame := client.getGame()

                if (client.getPlayer() != nil && client.getGame() != nil) {
                    thisPlayer := myGame.getPlayer(name)

                    if (thisPlayer != nil) {
                        if (thisPlayer.getTeam() == team) {
                            thisPlayer.setSendTo("team", myName, false)
                        } else {
                            thisPlayer.setSendTo("team", myName, true)
                        }
                    } else {
                        fmt.Printf("Name %s sent in teamCk doesn't exist\n", name)
                    }
                } else {
                    fmt.Printf("A client without a game/player is trying to check team\n")
                }
            case DEAD_CK:
                name := info[posInSlice + 1]
                dead, err := strconv.ParseBool(info[posInSlice + 2])
                myName := client.getPlayer().getName()
                myGame := client.getGame()

                if (err == nil) {
                    if (client.getPlayer() != nil && client.getGame() != nil) {
                        thisPlayer := myGame.getPlayer(name)

                        if (thisPlayer != nil) {
                            if (thisPlayer.getDead() == dead) {
                                thisPlayer.setSendTo("dead", myName, false)
                            } else {
                                thisPlayer.setSendTo("dead", myName, true)
                            }
                        } else {
                            fmt.Printf("Name %s sent in deadCk doesn't exist\n", name)
                        }
                    } else {
                        fmt.Printf("A client without a game/player is trying to check dead\n")
                    }
                } else {
                    fmt.Printf("Error parsing dead check: %v\n", err)
                    validBuffer = false
                }
            case DC_CK:
                name := info[posInSlice + 1]
                myName := client.getPlayer().getName()
                myGame := client.getGame()

                if (client.getPlayer() != nil && client.getGame() != nil) {
                    thisPlayer := myGame.getPlayer(name)

                    if (thisPlayer != nil) {
                        thisPlayer.setSendTo("dc", myName, false)
                    } else {
                        fmt.Printf("Name %s sent in dc check doesn't exist\n", name)
                    }
                } else {
                    fmt.Printf("A client without a game/player is trying to check dc\n")
                }
            default:
                //protects against invalid types
                fmt.Printf("Read invalid type: %s\n", bufType)
                validBuffer = false
            }

            if (validBuffer) {
                //if the buffer is valid it's mostly safe to assume that no other commands are "hidden" after the type
                posInSlice += posInc[bufType]
            } else {
                //if the data after the buffer is invalid this makes sure to only increment by one to check that case
                posInSlice++
            }
        }

        if (writeString != "") {
            write(addr, writeString)
        }

        if (printString != "") {
            printWrote(addr, printString)
        }
    }

    deleteAddr(addr)
    return
}

func broadcast() {
    for {
        pp := getPrintPeriodicals()
        addrs := getAddrs()

        for _, recAddr := range addrs {
            recClient, ok := getClient(recAddr)

            if (!ok) {
                continue
            }

            if (recClient.getSimClient() && recClient.getGame() != nil) {
                writeSimClientStuff(recAddr, recClient)
                continue
            }

            if (recClient.getGame() == nil || recClient.getPlayer() == nil || !recClient.getPlayer().getConnected()) {
                continue
            }

            //dealing with game elements
            numBorderPoints := len(recClient.getGame().getBorderPoints())

            for i := 0; i < numBorderPoints; i++ {
                if (recClient.getReceivingBP(i)) {
                    writeString := recClient.getGame().bpString(i)
                    write(recAddr, writeString)
                    printString := writeString
                    printWrote(recAddr, printString)
                    recClient.setReceivingBP(i, false)
                }
            }

            numRespawnPoints := len(recClient.getGame().getRespawnPoints())

            for i := 0; i < numRespawnPoints; i++ {
                if (recClient.getReceivingRP(i)) {
                    writeString := recClient.getGame().rpString(i)
                    write(recAddr, writeString)
                    printString := writeString
                    printWrote(recAddr, printString)
                    recClient.setReceivingRP(i, false)
                }
            }

            recPlayer := recClient.getPlayer()

            //don't send any player info if the recPlayer is in the background
            if (recPlayer.getBK()) {
                continue
            }

            //looping through all the sendPlayers
            for _, thisPlayer := range recClient.getGame().getPlayers() {
                if thisPlayer != recClient.getPlayer() {
                    writeString := ""
                    printString := ""

                    if val, ok := thisPlayer.getSendTo("dc", recPlayer.getName()); ok && val {
                        additionalString := fmt.Sprintf("%d:%s:", DC, thisPlayer.getName())
                        writeString += additionalString
                        printString += additionalString
                    }

                    //checking that they're in game (connected means in game)
                    if (thisPlayer.getConnected()) {
                        //sending location
                        additionalString := fmt.Sprintf("%d:%s:%s:%s:", LOC, thisPlayer.getName(), truncate(thisPlayer.getLat(), LocPlaces), truncate(thisPlayer.getLong(), LocPlaces))
                        writeString += additionalString

                        if (pp) {
                            printString += additionalString
                        }

                        sendTeam, stOk := thisPlayer.getSendTo("team", recPlayer.getName())

                        if stOk && sendTeam {
                            additionalString := fmt.Sprintf("%d:%s:%s:", TEAM, thisPlayer.getName(), thisPlayer.getTeam())
                            writeString += additionalString
                            printString += additionalString
                        }

                        if val, ok := thisPlayer.getSendTo("ward", recPlayer.getName()); ok && val && stOk && !sendTeam {
                            additionalString := fmt.Sprintf("%d:%s:%s:%s:", WARD, thisPlayer.getName(), truncate(thisPlayer.getWardLat(), LocPlaces), truncate(thisPlayer.getWardLong(), LocPlaces))
                            writeString += additionalString
                            printString += additionalString
                        }

                        if val, ok := thisPlayer.getSendTo("dead", recPlayer.getName()); ok && val {
                            additionalString := fmt.Sprintf("%d:%s:%t:", DEAD, thisPlayer.getName(), thisPlayer.getDead())
                            writeString += additionalString
                            printString += additionalString
                        }
                    }

                    if (writeString != "") {
                        write(recAddr, writeString)
                    }

                    if (printString != "") {
                        printWrote(recAddr, printString)
                    }
                }
            }

            //recClient.setReceivingInitial(false)
            recClient.setReceiving(false)
        }

        //writing to json file is here so that it's done regularly regardless of disconnects
        if (debug) {
            jsonFile, err := json.MarshalIndent(getMaster(), "", " ")

            if (err == nil) {
                err = ioutil.WriteFile("master.json", jsonFile, 0644)

                if (err != nil) {
                    fmt.Printf("Error writing json file: %v\n", err)
                }
            } else {
                fmt.Printf("Error converting master to json: %v\n", err)
            }
        }

        time.Sleep(1 * time.Second)
    }
}

func writeSimClientStuff(addr string, client *Client) {
    for _, thisName := range client.getGame().getPlayerNames() {
        if client.getGame().getPlayer(thisName) != client.getPlayer() {
            writeString := ""
            printString := ""

            //checking that they're in game (connected means in game)
            if (client.getGame().getPlayer(thisName).getConnected()) {
                //sending location
                thisLat, thisLong := client.getGame().getPlayer(thisName).getLoc()

                additionalString := fmt.Sprintf("%d:%s:%f:%f:", LOC, thisName, thisLat, thisLong)
                writeString += additionalString

                if (getPrintPeriodicals()) {
                    printString += additionalString
                }
            }

            if (writeString != "") {
                write(addr, writeString)
            }

            if (printString != "") {
                printWrote(addr, printString)
            }
        }
    }
}

func write(addr string, writeString string) {
    if (rand.Float64() > PacketLossChance) {
        addrObject, err := net.ResolveUDPAddr("udp", addr)

        if err != nil {
            fmt.Printf("Error resolving udp address: %v\n", err)
        } else {
            _, err := packetConn.WriteTo([]byte(writeString), addrObject)

            if err != nil {
                fmt.Printf("Error writing: %v\n", err)
            }
        }
    }
}

func printWrote(addr string, printString string) {
    var nameString string

    if client, ok := getClient(addr); ok {
        if (client.getPlayer() != nil) {
            nameString = client.getPlayer().getName()
        } else {
            nameString = "no name"
        }
    } else {
        nameString = "no client"
    }

    fmt.Printf("Wrote %s to %s at %s\n", printString, nameString, addr)
}

func printRead(client *Client, info []string, periodicals bool, addr string) {
    printString := ""
    posInSlice := 0

    for ;posInSlice < len(info) - 1; {
        bufType, err := convertBufType(info[posInSlice])

        if (err != nil) {
            fmt.Printf("buffer type convert to int error: %v\n", err)
            posInSlice++
            continue
        }

        if _, ok := posInc[bufType]; ok {
            if (periodicals || (bufType != HEART && bufType != LOC)) {
                for printPos := posInSlice; printPos < posInSlice + posInc[bufType] &&
                    printPos < len(info) - 1; printPos++ {
                    printString += info[printPos] + ":"
                }
            }

            posInSlice += posInc[bufType]
        } else {
            printString += fmt.Sprintf("%d:", bufType)
            posInSlice++
        }
    }

    if (printString != "") {
        var idString string

        if (client.getPlayer() != nil) {
            idString = client.getPlayer().getName()
        } else {
            idString = "no name"
        }

        fmt.Printf("Read %s from %s at %s\n", printString, idString, addr)
    }
}

func convertBufType(bufTypeStr string) (uint8, error) {
    bufTypeUint64, err := strconv.ParseUint(bufTypeStr, 10, 8)
    bufTypeUint8 := uint8(bufTypeUint64)
    return bufTypeUint8, err
}

func baseMaster() *Master {
    ret := &Master {
        Games: map[string]*Game {
            "Home": &Game {
                GameID: "Home",

                RespawnPoints: []*RespawnPoint {
                    &RespawnPoint {
                        Lat: 38.32410,
                        Long: -121.98119,
                    },
                },

                BorderPoints: []*Coord {
                    &Coord {
                        Lat: 38,
                        Long: -120,
                    },
                    &Coord {
                        Lat: 39,
                        Long: -119,
                    },
                    &Coord {
                        Lat: 40,
                        Long: -119,
                    },
                    &Coord {
                        Lat: 41,
                        Long: -120,
                    },
                    &Coord {
                        Lat: 40,
                        Long: -121,
                    },
                    &Coord {
                        Lat: 39,
                        Long: -121,
                    },
                },

                Players: map[string]*Player {},
            },

            /*"DeAnza": &Game {
                GameID: "DeAnza",

                RespawnPoints: []*RespawnPoint {
                    &RespawnPoint {
                        Lat: 37.32032,
                        Long: -122.04449,
                    },
                    &RespawnPoint {
                        Lat: 37.32024,
                        Long: -122.04502,
                    },
                    &RespawnPoint {
                        Lat: 37.31999,
                        Long: -122.04552,
                    },
                    &RespawnPoint {
                        Lat: 37.32111,
                        Long: -122.045907,
                    },
                    &RespawnPoint {
                        Lat: 37.32053,
                        Long: -122.04532,
                    },
                    &RespawnPoint {
                        Lat: 37.32124,
                        Long: -122.04510,
                    },
                },

                Players: map[string]*Player {},
            },*/
        },
    }

    return ret
}

func getMaster() *Master {
    mutex.Lock()
    defer mutex.Unlock()
    return master
}

func setMaster(masterToSet *Master) {
    mutex.Lock()
    defer mutex.Unlock()
    master = masterToSet
}

func getAddrs() []string {
    mutex.Lock()
    defer mutex.Unlock()
    addrs := make([]string, len(addrToClient))
    index := 0

    for addr, _ := range addrToClient {
        addrs[index] = addr
        index++
    }

    return addrs
}

func getAddrToClient() map[string]*Client {
    mutex.Lock()
    defer mutex.Unlock()
    return addrToClient
}

func setAddrToClient(newMap map[string]*Client) {
    mutex.Lock()
    defer mutex.Unlock()
    addrToClient = newMap
}

func getAddrToChannel() map[string]chan string {
    mutex.Lock()
    defer mutex.Unlock()
    return addrToChannel
}

func setAddrToChannel(newMap map[string]chan string) {
    mutex.Lock()
    defer mutex.Unlock()
    addrToChannel = newMap
}

func getChannel(addr string) chan string {
    mutex.Lock()
    defer mutex.Unlock()
    return addrToChannel[addr]
}

func setChannel(addr string, channel chan string) {
    mutex.Lock()
    defer mutex.Unlock()
    addrToChannel[addr] = channel
}

func setClient(addr string, client *Client) {
    mutex.Lock()
    defer mutex.Unlock()
    addrToClient[addr] = client
}

func getClient(addr string) (*Client, bool) {
    mutex.Lock()
    defer mutex.Unlock()
    client, ok := addrToClient[addr]
    return client, ok
}

func deleteAddr(addr string) {
    mutex.Lock()
    defer mutex.Unlock()
    delete(addrToClient, addr)
}

func getPrintPeriodicals() bool {
    mutex.Lock()
    defer mutex.Unlock()
    return printPeriodicals
}

func setPrintPeriodicals(val bool) {
    mutex.Lock()
    defer mutex.Unlock()
    printPeriodicals = val
}

func truncate(num float64, places int) string {
    tenPowerNum := math.Pow(10, float64(places))
    ret := strconv.FormatFloat(math.Round(num * tenPowerNum) / tenPowerNum, 'f', -1, 64)
    return ret
}
