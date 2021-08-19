Pod::Spec.new do |spec|
  spec.name         = 'GAVN'
  spec.version      = '{{.Version}}'
  spec.license      = { :type => 'GNU Lesser General Public License, Version 3.0' }
  spec.homepage     = 'https://github.com/AVNereum/go-AVNereum'
  spec.authors      = { {{range .Contributors}}
		'{{.Name}}' => '{{.Email}}',{{end}}
	}
  spec.summary      = 'iOS Avalanria Client'
  spec.source       = { :git => 'https://github.com/AVNereum/go-AVNereum.git', :commit => '{{.Commit}}' }

	spec.platform = :ios
  spec.ios.deployment_target  = '9.0'
	spec.ios.vendored_frameworks = 'Frameworks/GAVN.framework'

	spec.prepare_command = <<-CMD
    curl https://gAVNstore.blob.core.windows.net/builds/{{.Archive}}.tar.gz | tar -xvz
    mkdir Frameworks
    mv {{.Archive}}/GAVN.framework Frameworks
    rm -rf {{.Archive}}
  CMD
end
