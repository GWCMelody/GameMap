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
)

const (
    HostName = "10.0.1.128"
    //HostName = "127.0.0.1"
    Port = "8888"
    ConnType = "tcp"
)

//dictionary of conn objects to client objects
var connToClient map[*net.Conn]*Client
//the master json object
var master *Master
//this is explained below
var posInc map[string]int
var mutex sync.Mutex //lock this with actions regarding connToClient or printPeriodicals
var printPeriodicals bool

func main() {
    /*initializes the connToClient array. the design is kinda weird, i basically treat
    rlsServer itself as its own class with its own getters and setters. this way, i
    don't forget use mutex before accessing or changing any data since locking the
    mutex is built in (by the code, not by golang) to all getters and setters*/
    setConnToClient(make(map[*net.Conn]*Client))
    //sets print periodicals to true. you might want to set this to false by default by preference
    setPrintPeriodicals(true)

    //position increment. the amount of cells to increment in the string array after every command
    posInc = map[string]int {
        //explanations of commands are in the simulated client guide doc
        "hrt": 1,
        "connected": 1,
        "toggleRDL": 1,
        "togglePP": 1,
        "checkID": 2,
        "checkName": 2,
        "rec": 1,
        "team": 2,
        "loc": 3,
        "ward": 3,
        "dead": 2,
        "ret": 1,
    }

    //sets the default state of the master json
    master = baseMaster()
    //creates the listener (for connections, not for data)
    l, err := net.Listen(ConnType, HostName + ":" + Port)
    defer l.Close()

    //crashes if there's an error in creating the listener (for example, if another instance of the server is running)
    if err != nil {
        fmt.Printf("Error listening: %v\n", err.Error())
        os.Exit(1)
    }

    fmt.Printf("Listening on %s:%s\n", HostName, Port)
    //broadcast has its own forever loop so this function only needs to be called once
    go broadcast()

    //the forever loop that runs the entire server
    for {
        //check for connections (the code stops here if there are no connections i think)
        conn, err := l.Accept()
        fmt.Printf("connection detected\n")

        if err != nil {
            fmt.Printf("Error accepting: %v\n", err.Error())
            //think about not exiting for a failed connection
            os.Exit(1)
        }

        //if the connection is successful, create a new routine that listens for data from that connection
        go handleRequest(conn)
    }

    return
}

func handleRequest(conn net.Conn) {
    client := &(Client{})
    setClient(&conn, client)
    rdlEnabled := true

    for {
        buf := make([]byte, 1024)

        if (rdlEnabled) {
            conn.SetReadDeadline(time.Now().Local().Add(time.Second * time.Duration(30)))
        }

        _, err := conn.Read(buf)
        buf = bytes.Trim(buf, "\x00")
		content := string(buf)

        if err != nil {
            fmt.Printf("Error reading: %v\n", err.Error())
            client.playerDisconnectActions()
            return
        }

		info := strings.Split(content, ":")
        writeString := ""
        printString := ""
        posInSlice := 0
        pp := getPrintPeriodicals()
        printRead(client, info, pp)

        for ;posInSlice < len(info) - 1; {
            validBuffer := true
            bufType := info[posInSlice]

            //protects against valid types without enough data following them
            if (posInSlice + posInc[bufType] >= len(info)) {
                //it's ++ instead of break in the case that loc:toggleRDL: or smth is sent
                //this can never happen
                fmt.Printf("Valid type sent without enough data\n")
                posInSlice++
                continue
            }

            switch bufType {
            case "hrt":
                additionalString := "bt:"
                writeString += additionalString

                if (pp) {
                    printString += additionalString
                }
            case "connected": //why is connected used
            case "toggleRDL":
                rdlEnabled = !rdlEnabled

                if (!rdlEnabled) {
                    conn.SetReadDeadline(time.Time{})
                }
            case "togglePP":
                setPrintPeriodicals(!getPrintPeriodicals())
            case "checkID":
                readGameID := info[posInSlice + 1]

                if (readGameID != "") {
                    idTaken := master.checkIDTaken(readGameID)

                    if (!idTaken) { //fix this so that a new game isn't created when trying to join a nonexisting game
                        thisGame := &Game{}
                        thisGame.constructor()
                        thisGame.setGameID(readGameID)
                        master.addGame(thisGame)
                        client.setGame(thisGame)
                    } else {
                        client.setGame(master.getGame(readGameID))
                    }

                    additionalString := fmt.Sprintf("checkID:%t:", idTaken)
                    writeString += additionalString
                    printString += additionalString
                }
            case "checkName":
                var readName = info[posInSlice + 1]

                if (readName != "") {
                    if (client.getGame() != nil) {
                        nameTaken := client.getGame().checkNameTaken(readName)

                        if (!nameTaken) {
                            thisPlayer := &(Player{})
                            client.setPlayer(thisPlayer)
                            thisGame := client.getGame()
                            thisPlayer.constructor(thisGame.getPlayers())
                            thisPlayer.setName(readName)
                            thisGame.addPlayer(thisPlayer)
                        }

                        additionalString := fmt.Sprintf("checkName:%t:", nameTaken)
                        writeString += additionalString
                        printString += additionalString
                    } else {
                        fmt.Printf("A client without a game is trying to add a player\n")
                    }
                }
            case "rec":
                client.setReceiving(true)
            case "team":
                team := info[posInSlice + 1]

                if (client.getPlayer() != nil && client.getGame() != nil) {
                    thisPlayer := client.getPlayer()
                    players := client.getGame().getPlayers()

                    if (team != thisPlayer.getTeam()) {
                        thisPlayer.setWardLoc(200, 200)
                        thisPlayer.makeSendTrue("ward", players)
                    }

                    thisPlayer.setTeam(team)
                    thisPlayer.makeSendTrue("team", players)
                } else {
                    fmt.Printf("A client without a game/player is trying to set team\n")
                }
            case "loc":
                lat, err1 := strconv.ParseFloat(info[posInSlice + 1], 64)
                long, err2 := strconv.ParseFloat(info[posInSlice + 2], 64)

                if (err1 == nil && err2 == nil) {
                    if (client.getPlayer() != nil && client.getGame() != nil) {
                        if (client.getPlayer().getLat() == 0 && client.getPlayer().getLong() == 0) {
                            client.setReceivingInitial(true)
                        }

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
            case "ward":
                lat, err1 := strconv.ParseFloat(info[posInSlice + 1], 64)
                long, err2 := strconv.ParseFloat(info[posInSlice + 2], 64)

                if (err1 == nil && err2 == nil) {
                    if (client.getPlayer() != nil && client.getGame() != nil) {
                        client.getPlayer().setWardLoc(lat, long)
                        client.getPlayer().makeSendTrue("ward", client.getGame().getPlayers())
                    } else {
                        fmt.Printf("A client without a game/player is trying to set ward loc\n")
                    }
                } else {
                    fmt.Printf("Error parsing ward location. Lat error: %v; long error: %v\n", err1, err2)
                    validBuffer = false
                }
            case "dead":
                dead, err := strconv.ParseBool(info[posInSlice + 1])

                if (err == nil) {
                    if (client.getPlayer() != nil && client.getGame() != nil) {
                        client.getPlayer().setDead(dead)
                        client.getPlayer().makeSendTrue("dead", client.getGame().getPlayers())
                    } else {
                        fmt.Printf("A client without a game/player is trying to set dead\n")
                    }
                } else {
                    fmt.Printf("Error parsing dead: %v\n", err)
                    validBuffer = false
                }
            case "ret":
                client.playerDisconnectActions()
            default:
                //protects against invalid types
                fmt.Printf("Read invalid type: %s\n", bufType)
                validBuffer = false
            }

            if (validBuffer) {
                //if the buffer is valid it's mostly safe to assume that no other commands are "hidden" after the type
                posInSlice += posInc[bufType]
            } else {
                posInSlice++
            }
        }

        if (writeString != "") {
            write(&conn, client, writeString)
        }

        if (printString != "") {
            printWrote(client, printString)
        }
    }

    deleteConn(&conn)
    conn.Close()
    return
}

func broadcast() {
    for {
        for recConn, recClient := range getConnToClient() {
            if !recClient.getReceiving() || recClient.getGame() == nil || recClient.getPlayer() == nil {
                continue
            }

            //one time things for clients like respawn points, game settings, etc
            if (recClient.getReceivingInitial()) {
                writeString := recClient.getGame().rpString()
                printString := writeString
                write(recConn, recClient, writeString)
                printWrote(recClient, printString)
            }

            recPlayer := recClient.getPlayer()

            //things that have to do with other players
            for _, thisPlayer := range recClient.getGame().getPlayers() {
                //checking that their position isn't 0,0 ensures they are in game
                if thisPlayer != recClient.getPlayer() && thisPlayer.getLat() != 0 && thisPlayer.getLong() != 0 {
                    writeString := ""
                    printString := ""
                    pp := getPrintPeriodicals()

                    //receivingInitial is for players who just joined the game to get the current game state
                    //sendTo arrays is for players already in the game to broadcast their changed info to all other players
                    if recClient.getReceivingInitial() {
                        writeString = thisPlayer.initialPlayerString()
                        printString = writeString
                    } else {
                        if (thisPlayer.getConnected()) {
                            additionalString := fmt.Sprintf("loc:%s:%f:%f:", thisPlayer.getName(), thisPlayer.getLat(), thisPlayer.getLong())
                            writeString += additionalString

                            if (pp) {
                                printString += additionalString
                            }
                        }

                        //conn has to be before everything because when conn is true the client creates a new player
                        if val, ok := thisPlayer.getSendTo("conn", recPlayer.getName()); ok && val {
                            additionalString := fmt.Sprintf("conn:%s:%t:", thisPlayer.getName(), thisPlayer.getConnected())
                            writeString += additionalString
                            printString += additionalString
                            thisPlayer.setSendTo("conn", recPlayer.getName(), false)
                        }

                        //team has to be before ward so the ward is drawn with the team known
                        if val, ok := thisPlayer.getSendTo("team", recPlayer.getName()); ok && val {
                            additionalString := fmt.Sprintf("team:%s:%s:", thisPlayer.getName(), thisPlayer.getTeam())
                            writeString += additionalString
                            printString += additionalString
                            thisPlayer.setSendTo("team", recPlayer.getName(), false)
                        }

                        if val, ok := thisPlayer.getSendTo("ward", recPlayer.getName()); ok && val {
                            additionalString := fmt.Sprintf("ward:%s:%f:%f:", thisPlayer.getName(), thisPlayer.getWardLat(), thisPlayer.getWardLong())
                            writeString += additionalString
                            printString += additionalString
                            thisPlayer.setSendTo("ward", recPlayer.getName(), false)
                        }

                        if val, ok := thisPlayer.getSendTo("dead", recPlayer.getName()); ok && val {
                            additionalString := fmt.Sprintf("dead:%s:%t:", thisPlayer.getName(), thisPlayer.getDead())
                            writeString += additionalString
                            printString += additionalString
                            thisPlayer.setSendTo("dead", recPlayer.getName(), false)
                        }
                    }

                    if (writeString != "") {
                        write(recConn, recClient, writeString)
                    }

                    if (printString != "") {
                        printWrote(recClient, printString)
                    }
                }
            }

            recClient.setReceivingInitial(false)
            recClient.setReceiving(false)
        }

        //writing to json file is here so that it's done regularly regardless of disconnects
        jsonFile, err := json.MarshalIndent(*master, "", " ")

        if (err == nil) {
            err = ioutil.WriteFile("master.json", jsonFile, 0644)

            if (err != nil) {
                fmt.Printf("Error writing json file: %v\n", err)
            }
        } else {
            fmt.Printf("Error converting master to json: %v\n", err)
        }

        time.Sleep(1 * time.Second)
    }
}

func write(conn *net.Conn, client *Client, writeString string) {
    _, err := (*conn).Write([]byte(writeString))

    if err != nil {
        fmt.Printf("Error writing: %v\n", err)
    }
}

func printWrote(client *Client, printString string) {
    var idString string

    if (client.getPlayer() != nil) {
        idString = client.getPlayer().getName()
    } else {
        idString = "no name"
    }

    fmt.Printf("Wrote %s to %s\n", printString, idString)
}

func printRead(client *Client, info []string, periodicals bool) {
    printString := ""
    posInSlice := 0

    for ;posInSlice < len(info) - 1; {
        bufType := info[posInSlice]

        if _, ok := posInc[bufType]; ok {
            if (periodicals || (bufType != "hrt" && bufType != "loc" && bufType != "rec")) {
                for printPos := posInSlice; printPos < posInSlice + posInc[bufType] &&
                    printPos < len(info) - 1; printPos++ {
                    printString += info[printPos] + ":"
                }
            }

            posInSlice += posInc[bufType]
        } else {
            printString += bufType + ":"
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

        fmt.Printf("Read %s from %s\n", printString, idString)
    }
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

    /*blueDummy := &Player{}
    blueDummy.constructor(ret.getGame("Home").getPlayers())
    blueDummy.setName("blueDummy")
    blueDummy.setTeam("blue")
    blueDummy.setLoc(37.32440, -121.98119)
    ret.getGame("Home").addPlayer(blueDummy)*/

    return ret
}

func getConnToClient() map[*net.Conn]*Client {
    mutexLock()
    return connToClient
}

func setConnToClient(newMap map[*net.Conn]*Client) {
    mutexLock()
    connToClient = newMap
}

func setClient(conn *net.Conn, client *Client) {
    mutexLock()
    connToClient[conn] = client
}

func deleteConn(conn *net.Conn) {
    mutexLock()
    delete(connToClient, conn)
}

func getPrintPeriodicals() bool {
    mutexLock()
    return printPeriodicals
}

func setPrintPeriodicals(val bool) {
    mutexLock()
    printPeriodicals = val
}

func mutexLock() {
    mutex.Lock()
    defer mutex.Unlock()
}