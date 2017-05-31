package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	"encoding/json"
	"regexp"
)

var logger = shim.NewLogger("CLDChaincode")

const   SENDER    =  "sender"
const   SHIPPER    =  "shipper"
const   RECEIVER  =  "receiver"

const   STATE_READY_TO_SEND =  0
const   STATE_SHIPPING      =  1
const   STATE_RECEIVED      =  2

type  SimpleChaincode struct {
}

type Container struct {
	Id              string `json:"id"`
  Weight          int    `json:"weight"`
	Owner           string `json:"owner"`
	Status          int    `json:"status"`
	V5cID           string `json:"v5cID"`
  Temperature     int    `json:"temperature"`
}

type V5C_Holder struct {
	V5Cs 	[]string `json:"v5cs"`
}

type User_and_eCert struct {
	Identity string `json:"identity"`
	eCert string `json:"ecert"`
}

func (t *SimpleChaincode) Init(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {

	//Args
	//				0
	//			peer_address

	var v5cIDs V5C_Holder

	bytes, err := json.Marshal(v5cIDs)

    if err != nil { return nil, errors.New("Error creating V5C_Holder record") }

	err = stub.PutState("v5cIDs", bytes)

	for i:=0; i < len(args); i=i+2 {
		t.add_ecert(stub, args[i], args[i+1])
	}

	return nil, nil
}

func (t *SimpleChaincode) get_ecert(stub shim.ChaincodeStubInterface, name string) ([]byte, error) {

	ecert, err := stub.GetState(name)

	if err != nil { return nil, errors.New("Couldn't retrieve ecert for user " + name) }

	return ecert, nil
}

func (t *SimpleChaincode) add_ecert(stub shim.ChaincodeStubInterface, name string, ecert string) ([]byte, error) {


	err := stub.PutState(name, []byte(ecert))

	if err == nil {
		return nil, errors.New("Error storing eCert for user " + name + " identity: " + ecert)
	}

	return nil, nil

}

func (t *SimpleChaincode) get_username(stub shim.ChaincodeStubInterface) (string, error) {

    username, err := stub.ReadCertAttribute("username");
	if err != nil { return "", errors.New("Couldn't get attribute 'username'. Error: " + err.Error()) }
	return string(username), nil
}

func (t *SimpleChaincode) check_affiliation(stub shim.ChaincodeStubInterface) (string, error) {
    affiliation, err := stub.ReadCertAttribute("role");
	if err != nil { return "", errors.New("Couldn't get attribute 'role'. Error: " + err.Error()) }
	return string(affiliation), nil

}

func (t *SimpleChaincode) get_caller_data(stub shim.ChaincodeStubInterface) (string, string, error){

	user, err := t.get_username(stub)

    // if err != nil { return "", "", err }

	// ecert, err := t.get_ecert(stub, user);

    // if err != nil { return "", "", err }

	affiliation, err := t.check_affiliation(stub);

  if err != nil { return "", "", err }

	return user, affiliation, nil
}

func (t *SimpleChaincode) retrieve_v5c(stub shim.ChaincodeStubInterface, v5cID string) (Container, error) {

	var v Container

	bytes, err := stub.GetState(v5cID);

	if err != nil {	fmt.Printf("RETRIEVE_V5C: Failed to invoke vehicle_code: %s", err); return v, errors.New("RETRIEVE_V5C: Error retrieving vehicle with v5cID = " + v5cID) }

	err = json.Unmarshal(bytes, &v);

    if err != nil {	fmt.Printf("RETRIEVE_V5C: Corrupt vehicle record "+string(bytes)+": %s", err); return v, errors.New("RETRIEVE_V5C: Corrupt vehicle record"+string(bytes))	}

	return v, nil
}


func (t *SimpleChaincode) save_changes(stub shim.ChaincodeStubInterface, v Container) (bool, error) {

	bytes, err := json.Marshal(v)

	if err != nil { fmt.Printf("SAVE_CHANGES: Error converting vehicle record: %s", err); return false, errors.New("Error converting vehicle record") }

	err = stub.PutState(v.V5cID, bytes)

	if err != nil { fmt.Printf("SAVE_CHANGES: Error storing vehicle record: %s", err); return false, errors.New("Error storing vehicle record") }

	return true, nil
}

func (t *SimpleChaincode) Invoke(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	logger.Debug("function: ", function)
	caller, caller_affiliation, err := t.get_caller_data(stub)

	if err != nil { return nil, errors.New("Error retrieving caller information")}

	if function == "create_container" {
				logger.Debug("call create_container: ")
        return t.create_container(stub, caller, caller_affiliation, args[0])
	} else if function == "ping" {
        return t.ping(stub)
  } else { 																				// If the function is not a create then there must be a car so we need to retrieve the car.
		argPos := 1

		v, err := t.retrieve_v5c(stub, args[argPos])

    if err != nil { fmt.Printf("INVOKE: Error retrieving v5c: %s", err); return nil, errors.New("Error retrieving v5c") }

    if strings.Contains(function, "update") == false && function != "scrap_vehicle"    {

  		if function == "sender_to_shipper" { return t.sender_to_shipper(stub, v, caller, caller_affiliation, args[0], "manufacturer")
  		} else if  function == "shipper_to_receiver"   { return t.shipper_to_receiver(stub, v, caller, caller_affiliation, args[0], "private")
  		}

		} else if function == "update_temperature"  { return t.update_temperature(stub, v, caller, caller_affiliation, args[0])
		} else if function == "update_id"  { return t.update_id(stub, v, caller, caller_affiliation, args[0])
    } else if function == "update_weight"  { return t.update_weight(stub, v, caller, caller_affiliation, args[0])
    }
		return nil, errors.New("Function of the name "+ function +" doesn't exist.")

	}
}

func (t *SimpleChaincode) create_container(stub shim.ChaincodeStubInterface, caller string, caller_affiliation string, v5cID string) ([]byte, error) {
	var v Container

	v5c_ID         := "\"v5cID\":\""+v5cID+"\", "
	id             := "\"Id\": \"UNDEFINED\", "
	owner          := "\"Owner\":\""+caller+"\", "
	status         := "\"Status\":0, "
  weight         := "\"Weight\":0, "
  temperature    := "\"Temperature\":0, "

	container_json := "{"+v5c_ID+id+owner+status+weight+temperature+"}" 	// Concatenates the variables to create the total JSON object
	logger.Debug("call create_container: ", container_json)
	matched, err := regexp.Match("^[A-z][A-z][0-9]{7}", []byte(v5cID))  				// matched = true if the v5cID passed fits format of two letters followed by seven digits

	if err != nil { fmt.Printf("CREATE_VEHICLE: Invalid v5cID: %s", err); return nil, errors.New("Invalid v5cID") }
	logger.Debug("valid v5c")

	if 	v5c_ID  == "" 	 || matched == false    {
			fmt.Printf("CREATE_VEHICLE: Invalid v5cID provided");
			logger.Debug("CREATE_VEHICLE: Invalid v5cID provided")
			return nil, errors.New("Invalid v5cID provided")
	}

	err = json.Unmarshal([]byte(container_json), &v)							// Convert the JSON defined above into a vehicle object for go

	if err != nil { return nil, errors.New("Invalid JSON object") }
	logger.Debug("json")
	record, err := stub.GetState(v.V5cID) 								// If not an error then a record exists so cant create a new car with this V5cID as it must be unique

  if record != nil { return nil, errors.New("Container already exists") }
	logger.Debug("exists")
	if 	caller_affiliation != SENDER {							// Only the regulator can create a new v5
		logger.Debug("permission denied")
		return nil, errors.New(fmt.Sprintf("Permission Denied. create_vehicle. %v === %v", caller_affiliation, SENDER))
	}
	logger.Debug("save")
	_, err  = t.save_changes(stub, v)

	if err != nil { fmt.Printf("CREATE_CONTAINER: Error saving changes: %s", err); return nil, errors.New("Error saving changes") }

	bytes, err := stub.GetState("v5cIDs")

	if err != nil { return nil, errors.New("Unable to get v5cIDs") }

	var v5cIDs V5C_Holder

	err = json.Unmarshal(bytes, &v5cIDs)

	if err != nil {	return nil, errors.New("Corrupt V5C_Holder record") }

	v5cIDs.V5Cs = append(v5cIDs.V5Cs, v5cID)


	bytes, err = json.Marshal(v5cIDs)

	if err != nil { fmt.Print("Error creating V5C_Holder record") }

	err = stub.PutState("v5cIDs", bytes)

	if err != nil { return nil, errors.New("Unable to put the state") }

	return nil, nil

}

func (t *SimpleChaincode) sender_to_shipper(stub shim.ChaincodeStubInterface, v Container, caller string, caller_affiliation string, recipient_name string, recipient_affiliation string) ([]byte, error) {

	if  v.Status				== STATE_READY_TO_SEND	&&
			v.Owner					== caller			&&
			caller_affiliation		== SENDER		&&
			recipient_affiliation	== SHIPPER	{		// If the roles and users are ok

					v.Owner  = recipient_name		// then make the owner the new owner
					v.Status = STATE_SHIPPING			// and mark it in the state of manufacture

	} else {									// Otherwise if there is an error
		fmt.Printf("SENDER_TO_SHIPPER: Permission Denied");
    return nil, errors.New(fmt.Sprintf("Permission Denied. sender_to_shipper. %v %v === %v, %v === %v, %v === %v, %v === %v", v, v.Status, STATE_READY_TO_SEND, v.Owner, caller, caller_affiliation, SENDER, recipient_affiliation, SHIPPER))


	}

	_, err := t.save_changes(stub, v)						// Write new state

  if err != nil {	fmt.Printf("SENDER_TO_SHIPPER: Error saving changes: %s", err); return nil, errors.New("Error saving changes")	}

	return nil, nil									// We are Done

}


func (t *SimpleChaincode) shipper_to_receiver(stub shim.ChaincodeStubInterface, v Container, caller string, caller_affiliation string, recipient_name string, recipient_affiliation string) ([]byte, error) {

	if  v.Status				== STATE_SHIPPING	&&
			v.Owner					== caller			&&
			caller_affiliation		== SHIPPER		&&
			recipient_affiliation	== RECEIVER	{		// If the roles and users are ok

					v.Owner  = recipient_name		// then make the owner the new owner
					v.Status = STATE_RECEIVED			// and mark it in the state of manufacture

	} else {									// Otherwise if there is an error
		fmt.Printf("SHIPPER_TO_RECEIVER: Permission Denied");
    return nil, errors.New(fmt.Sprintf("Permission Denied. shipper_to_receiver. %v %v === %v, %v === %v, %v === %v, %v === %v", v, v.Status, STATE_SHIPPING, v.Owner, caller, caller_affiliation, SHIPPER, recipient_affiliation, RECEIVER))


	}

	_, err := t.save_changes(stub, v)						// Write new state

  if err != nil {	fmt.Printf("SHIPPER_TO_RECEIVER: Error saving changes: %s", err); return nil, errors.New("Error saving changes")	}

	return nil, nil									// We are Done

}

func (t *SimpleChaincode) update_temperature(stub shim.ChaincodeStubInterface, v Container, caller string, caller_affiliation string, new_value string) ([]byte, error) {
	new_temp, err := strconv.Atoi(string(new_value))

	if err != nil { return nil, errors.New("Invalid value passed for new Temperature") }

	v.Temperature = new_temp

	_, err = t.save_changes(stub, v)

	if err != nil { fmt.Printf("UPDATE_TEMPERATURE: Error saving changes: %s", err); return nil, errors.New("Error saving changes") }

	return nil, nil

}

func (t *SimpleChaincode) update_id(stub shim.ChaincodeStubInterface, v Container, caller string, caller_affiliation string, new_value string) ([]byte, error) {
	v.Id = new_value

	_, err := t.save_changes(stub, v)

	if err != nil { fmt.Printf("UPDATE_ID: Error saving changes: %s", err); return nil, errors.New("Error saving changes") }

	return nil, nil

}

func (t *SimpleChaincode) update_weight(stub shim.ChaincodeStubInterface, v Container, caller string, caller_affiliation string, new_value string) ([]byte, error) {

	new_weight, err := strconv.Atoi(string(new_value))

	if err != nil { return nil, errors.New("Invalid value passed for new Weight") }

	v.Weight = new_weight

	_, err = t.save_changes(stub, v)

	if err != nil { fmt.Printf("UPDATE_WEIGHT: Error saving changes: %s", err); return nil, errors.New("Error saving changes") }

	return nil, nil

}

func (t *SimpleChaincode) ping(stub shim.ChaincodeStubInterface) ([]byte, error) {
	return []byte("Hello, world!"), nil
}


func (t *SimpleChaincode) Query(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {

	caller, caller_affiliation, err := t.get_caller_data(stub)
	if err != nil { fmt.Printf("QUERY: Error retrieving caller details", err); return nil, errors.New("QUERY: Error retrieving caller details: "+err.Error()) }

    logger.Debug("function: ", function)
    logger.Debug("caller: ", caller)
    logger.Debug("affiliation: ", caller_affiliation)

	if function == "get_container_details" {
		if len(args) != 1 { fmt.Printf("Incorrect number of arguments passed"); return nil, errors.New("QUERY: Incorrect number of arguments passed") }
		v, err := t.retrieve_v5c(stub, args[0])
		if err != nil { fmt.Printf("QUERY: Error retrieving v5c: %s", err); return nil, errors.New("QUERY: Error retrieving v5c "+err.Error()) }
		return t.get_container_details(stub, v, caller, caller_affiliation)
	} else if function == "check_unique_v5c" {
		return t.check_unique_v5c(stub, args[0], caller, caller_affiliation)
	} else if function == "get_containers" {
		return t.get_containers(stub, caller, caller_affiliation)
	} else if function == "get_ecert" {
		return t.get_ecert(stub, args[0])
	} else if function == "ping" {
		return t.ping(stub)
	}

	return nil, errors.New("Received unknown function invocation " + function)

}

func (t *SimpleChaincode) get_container_details(stub shim.ChaincodeStubInterface, v Container, caller string, caller_affiliation string) ([]byte, error) {

	bytes, err := json.Marshal(v)

  if err != nil { return nil, errors.New("GET_VEHICLE_DETAILS: Invalid vehicle object") }

	if 		v.Owner				== caller		||
			caller_affiliation	== SENDER	{

					return bytes, nil
	} else {
																return nil, errors.New("Permission Denied. get_vehicle_details")
	}

}

func (t *SimpleChaincode) check_unique_v5c(stub shim.ChaincodeStubInterface, v5c string, caller string, caller_affiliation string) ([]byte, error) {
	_, err := t.retrieve_v5c(stub, v5c)
	if err == nil {
		return []byte("false"), errors.New("V5C is not unique")
	} else {
		return []byte("true"), nil
	}
}

func (t *SimpleChaincode) get_containers(stub shim.ChaincodeStubInterface, caller string, caller_affiliation string) ([]byte, error) {
	bytes, err := stub.GetState("v5cIDs")

  if err != nil { return nil, errors.New("Unable to get v5cIDs") }

	var v5cIDs V5C_Holder

	err = json.Unmarshal(bytes, &v5cIDs)

	if err != nil {	return nil, errors.New("Corrupt V5C_Holder") }

	result := "["

	var temp []byte
	var v Container

	for _, v5c := range v5cIDs.V5Cs {

		v, err = t.retrieve_v5c(stub, v5c)

		if err != nil {return nil, errors.New("Failed to retrieve V5C")}

		temp, err = t.get_container_details(stub, v, caller, caller_affiliation)

		if err == nil {
			result += string(temp) + ","
		}
	}

	if len(result) == 1 {
		result = "[]"
	} else {
		result = result[:len(result)-1] + "]"
	}

	return []byte(result), nil
}

func main() {

	err := shim.Start(new(SimpleChaincode))

	if err != nil { fmt.Printf("Error starting Chaincode: %s", err) }
}
