package main

import (
	"bufio"
	"encoding/base64"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
  
	"github.com/mitchellh/go-homedir"
)

func Usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s -url [URL] [-target [TARGET]]\n", os.Args[0])
	flag.PrintDefaults()
	os.Exit(1)
}

func ReplaceAllStringSubmatchFunc(re *regexp.Regexp, str string, repl func([]string) string) string {
	result := ""
	lastIndex := 0
	for _, v := range re.FindAllSubmatchIndex([]byte(str), -1) {
		groups := []string{}
		for i := 0; i < len(v) && v[i+1] > -1; i += 2 {
			groups = append(groups, str[v[i]:v[i+1]])
		}
		result += str[lastIndex:v[0]] + repl(groups)
		lastIndex = v[1]
	}
	return result + str[lastIndex:]
}

func MakeAbsolutePath(u *url.URL, s string, isRelative bool) string {
	if (strings.HasPrefix(s, "http:") || strings.HasPrefix(s, "https:")) {
		// This is already an absolute path.
		return s
	}
	
	if (isRelative) {
		return fmt.Sprintf("%s://%s%s%s", u.Scheme, u.Host, u.Path, s)
	} else {
		return fmt.Sprintf("%s://%s%s", u.Scheme, u.Host, s)
	}
}

func DeleteEmptySlices(s []string) []string {
	var r []string
	for _, str := range s {
		if str != "" {
			r = append(r, str)
		}
	}
	return r
}

func main() {
	homedir, err := homedir.Dir()
	if err != nil {
		fmt.Printf("Error trying to determine your home directory: %s\n", err)
		os.Exit(1)
	}
  
	var urlToGet string
	var target string
  
	flag.StringVar(&urlToGet, "url", "", "URL to download.")
	flag.StringVar(&target, "target", string(homedir), "Your target folder.")
	flag.Parse()
  
	if urlToGet == "" {
		Usage()
	}
  
	// Some parsing:
	u, err := url.Parse(urlToGet)
	if err != nil {
		fmt.Printf("Error parsing '%s': %s\n", urlToGet, err)
		os.Exit(1)
	}
  
	resp, err := http.Get(urlToGet)
	if err != nil {
		fmt.Printf("Error trying to download '%s': %s\n", urlToGet, err)
		os.Exit(1)
	}
  
	defer resp.Body.Close()
  
	respData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error trying to read the response data: %s\n", err)
		os.Exit(1)
	}

	respString := string(respData)
	targetFileName := fmt.Sprintf("%s/[%s] %s.htm", target, u.Host, strings.Replace(u.Path, "/", "_", -1))
	
	fmt.Printf("Downloading '%s' to '%s'...\n", urlToGet, targetFileName)

	// 1. Replace images in <respString>.
	fmt.Println("Converting images.")
	reImg := regexp.MustCompile("(<img (.*?)(src=\"([^\"]+)\")(.*?)>)")
	respStringWithNoImages := ReplaceAllStringSubmatchFunc(reImg, respString, func(groups []string) string {
		// groups[1] is an image, groups[3] is the src parameter,
		// groups[4] is the src path.
		// If groups[4] begins with /, it is a relative path to u.Host,
		// if it does not, it is a relative path to u.Path.
		// Replace groups[3] by a data URL parameter anyway.
		isRelative := true
		if (strings.HasPrefix(groups[4], "/")) {
			isRelative = false
		}
	  
		var imageType string
		imagePath := MakeAbsolutePath(u, groups[4], isRelative) 
	  
		if (strings.HasSuffix(groups[4], ".png")) {
			imageType = "image/png"
		} else if (strings.HasSuffix(groups[4], ".jpg") || strings.HasSuffix(groups[4], ".jpeg")) {
			imageType = "image/jpeg"
		} else if (strings.HasSuffix(groups[4], ".gif")) {
			imageType = "image/gif"
		} else {
			// Unknown type.
			fmt.Printf("Skipping '%s': unknown file type\n", imagePath)
			return fmt.Sprintf("<em>[MISSING: %s]</em>", imagePath)
		}
	  
		// Download and convert:
		img, imgerr := http.Get(imagePath)
		if imgerr != nil {
			// Skip this image.
			fmt.Printf("Skipping '%s': %s\n", imagePath, imgerr)
			return fmt.Sprintf("<em>[MISSING: %s]</em>", imagePath)
		}
		defer img.Body.Close()
	  
		reader := bufio.NewReader(img.Body)
		content, _ := ioutil.ReadAll(reader)
		encoded := base64.StdEncoding.EncodeToString(content)

		// Keep the parts before and after "src=" for the result.
		return fmt.Sprintf("<img %s src=\"data:image/%s;base64,%s\" %s />", groups[2], imageType, encoded, groups[5])
	})
	
	// 3. Inline CSS and JS in <respStringWithNoImages>.
	fmt.Println("Converting CSS.")
	reImgCss := regexp.MustCompile("(?:<link (.*?rel=\"stylesheet\".*?href=\"([^\"]+)\".*?|.*?href=\"([^\"]+)\".*?rel=\"stylesheet\".*?)>)")
	respStringWithNoCSS := ReplaceAllStringSubmatchFunc(reImgCss, respStringWithNoImages, func(groups []string) string {
		// The last non-empty item in groups[] is the path.
		groups = DeleteEmptySlices(groups)
		lastItem := groups[len(groups)-1]
		
		isRelative := true
		if (strings.HasPrefix(lastItem, "/")) {
			isRelative = false
		}
		
		cssPath := MakeAbsolutePath(u, lastItem, isRelative)
		
		// Download and append:
		css, csserr := http.Get(cssPath)
		if csserr != nil {
			// Skip this stylesheet.
			fmt.Printf("Skipping '%s': %s\n", cssPath, csserr)
			return groups[0]
		}
		defer css.Body.Close()
		
		reader := bufio.NewReader(css.Body)
		content, _ := ioutil.ReadAll(reader)
		
		return fmt.Sprintf("\n<style type='text/css'>%s</style>\n", content)
	})
	
	fmt.Println("Converting JavaScript.")
	reImgJs := regexp.MustCompile("(?:<script [^>]*?src=\"([^\"]+)\")[^>]*?>")
	respStringWithNoExternalResources := ReplaceAllStringSubmatchFunc(reImgJs, respStringWithNoCSS, func(groups []string) string {
		// The last item in groups[] is the path.
		lastItem := groups[len(groups)-1]
		
		isRelative := true
		if (strings.HasPrefix(lastItem, "/")) {
			isRelative = false
		}
		
		jsPath := MakeAbsolutePath(u, lastItem, isRelative)
		
		// Download and append:
		js, jserr := http.Get(jsPath)
		if jserr != nil {
			// Skip this script.
			fmt.Printf("Skipping '%s': %s\n", jsPath, jserr)
			return groups[0]
		}
		defer js.Body.Close()
		
		reader := bufio.NewReader(js.Body)
		content, _ := ioutil.ReadAll(reader)
		
		return fmt.Sprintf("\n<script language='text/javascript'>%s</script>\n", content)
	})
  
	// 4. Write <respStringWithNoExternalResources> into <targetFileName>.
	f, err := os.Create(targetFileName)
	if err != nil {
		fmt.Printf("Could not create the target file '%s': %s\n", targetFileName, err)
		os.Exit(1)
	}
	defer f.Close()
  
	createFile, err := f.WriteString(respStringWithNoExternalResources)  
	if err != nil {
		fmt.Printf("Could not write to the target file '%s': %s\n", targetFileName, err)
		os.Exit(1)
	}
  
	fmt.Printf("Done. Wrote %d bytes.\n", createFile)
}