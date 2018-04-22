//
//  NicknameLabel.swift
//  GameMap
//
//  Created by Melody Lee on 4/22/18.
//  Copyright © 2018 Hackathon Event. All rights reserved.
//

import UIKit

class NicknameLabel: UIViewController {
    
    
    @IBOutlet weak var label: UILabel!
    
    var nickname = String()
    
    override func viewDidLoad() {
        super.viewDidLoad()
        // Do any additional setup after loading the view, typically from a nib.
        label.text = nickname
    }
    
    //MARK: Actions
    
}
