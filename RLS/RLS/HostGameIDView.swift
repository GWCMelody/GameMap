//
//  HostGameIDView.swift
//  
//
//  Created by Melody Lee on 8/1/18.
//

import Foundation
import UIKit
import FirebaseFirestore

class HostGameIDView : UIViewController {
    @IBOutlet weak var HostGameID: UILabel!
    @IBAction func NextButton(_ sender: Any) {
        print("Next Button clicked")
        self.performSegue(withIdentifier: "EnterNicknameSegue", sender: self)
    }
    
    override func viewDidLoad() {
        super.viewDidLoad()
        HostGameID.text = "Game ID: " + gameID
        
        //start step function timer
        timer.invalidate()
        timer = Timer.scheduledTimer(timeInterval: 1, target: self, selector: #selector(step), userInfo: nil, repeats: true)
    }
    
<<<<<<< HEAD
    func locationManager(_ manager: CLLocationManager, didUpdateLocations locations: [CLLocation]) {
        let location = locations[0] //the latest location
        
        if (!once){
            let span: MKCoordinateSpan = MKCoordinateSpan.init(latitudeDelta: 0.01, longitudeDelta: 0.01)
            let myLocation = CLLocationCoordinate2DMake(location.coordinate.latitude, location.coordinate.longitude)
            let region:MKCoordinateRegion = MKCoordinateRegion.init(center: myLocation, span: span)
            mapView.setRegion(region, animated: true)
            self.mapView.showsUserLocation = true
            once = true
        }
    }
    
    //ping with long press
    @objc func handleTap(gestureRecognizer: UITapGestureRecognizer) {
        let touchLocation = gestureRecognizer.location(in: mapView)
        let locationCoordinate = mapView.convert(touchLocation,toCoordinateFrom: mapView)
        addBoord(boord: CLLocation(latitude: locationCoordinate.latitude, longitude: locationCoordinate.longitude))
    }
    
    func addBoord(boord: CLLocation) {
        mapView.removeOverlay(border)
        boords.append(boord)
        border = BorderOverlay(vertices: boords)
        mapView.addOverlay(border)
    }
    
    @IBAction func NextButton(_ sender: Any) {
        print("Next Button clicked")
        //stop the location manager
        manager.stopUpdatingLocation()
        //send game data like rp and boords
        for boord in boords {
            //make boord object later
            networking.sendBoord(boord: boord)
        }
        
        //go to nickname view
        self.performSegue(withIdentifier: "EnterNicknameSegue", sender: self)
    }
    
=======
>>>>>>> refs/remotes/origin/master
    @objc func step() {
        //send heartbeat
        networking.readAllData()
        networking.sendHeartbeat()
    }
    
    override func didReceiveMemoryWarning() {
        
    }
    
    override func prepare(for segue: UIStoryboardSegue, sender: Any?) {
        //var DestViewController : NicknameTFView = segue.destination as! NicknameTFView
    }
}
